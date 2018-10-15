package main

import (
	"encoding/json"
	"encoding/xml"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

type XMLResult struct {
	XMLName xml.Name  `xml:"root"`
	Rows    []RowUser `xml:"row"`
}

type RowUser struct {
	Id        int    `xml:"id"`
	FirstName string `xml:"first_name"`
	LastName  string `xml:"last_name"`
	Age       int    `xml:"age"`
	About     string `xml:"about"`
	Gender    string `xml:"gender"`
}

func SearchServer(rw http.ResponseWriter, r *http.Request) {
	var err error

	if r.Header.Get("AccessToken") != "FindUsers-Test-Token" {
		http.Error(rw, "Incorrect access token", http.StatusUnauthorized)
		return
	}

	query := strings.ToLower(r.URL.Query().Get("query"))
	limit := 25
	if r.URL.Query().Get("limit") != "" {
		limit, err = strconv.Atoi(r.URL.Query().Get("limit"))
		if err != nil {
			http.Error(rw, "", http.StatusBadRequest)
			return
		}
	}
	if limit == 1 {
		rw.Write(nil)
		return
	}
	offset := 0
	if r.URL.Query().Get("offset") != "" {
		offset, _ = strconv.Atoi(r.URL.Query().Get("offset"))
	}
	orderBy := 0
	if r.URL.Query().Get("order_by") != "" {
		orderBy, _ = strconv.Atoi(r.URL.Query().Get("order_by"))
	}

	orderField := "id"
	if r.URL.Query().Get("order_field") != "" {
		orderField = r.URL.Query().Get("order_field")
	}

	data, _ := ioutil.ReadFile("./dataset.xml")
	rowUsers := XMLResult{}
	xml.Unmarshal(data, &rowUsers)

	resUsers := make([]User, 0, len(rowUsers.Rows))
	for _, rowUser := range rowUsers.Rows {
		resUser := User{}
		resUser.Id = rowUser.Id
		resUser.Name = rowUser.FirstName + " " + rowUser.LastName
		resUser.Gender = rowUser.Gender
		resUser.Age = rowUser.Age
		resUser.About = rowUser.About
		resUsers = append(resUsers, resUser)
	}

	if query != "" {
		resUsers = queryProcess(resUsers, query)
	}

	if offset > len(resUsers) {
		http.Error(rw, "Server error", http.StatusInternalServerError)
		return
	}

	if offset < len(resUsers) {
		resUsers = resUsers[offset:]
	}
	if limit < len(resUsers) {
		resUsers = resUsers[:limit]
	}

	switch orderBy {
	case 0:
	case 1:
	case -1:
		// TODO order
		break
	default:
		http.Error(rw, "", http.StatusBadRequest)
	}

	switch orderField {
	case "id":
	case "age":
	case "name":
		// TODO order
		break
	case "about":
		{
			err := SearchErrorResponse{
				Error: "ErrorBadOrderField",
			}
			res, _ := json.Marshal(err)
			rw.WriteHeader(http.StatusBadRequest)
			rw.Write(res)
			return
		}
	default:
		{
			err := SearchErrorResponse{
				Error: "Unknown error",
			}
			res, _ := json.Marshal(err)
			rw.WriteHeader(http.StatusBadRequest)
			rw.Write(res)
			return
		}
	}

	result, _ := json.Marshal(resUsers)
	rw.Write(result)
}

func BrokenServer(rw http.ResponseWriter, r *http.Request) {
	time.Sleep(2 * time.Second)
}

func TestSearchClient_FindUsersClientError(t *testing.T) {
	sc := &SearchClient{
		URL: "localhost:9000",
	}

	_, err := sc.FindUsers(SearchRequest{})
	if err == nil {
		t.Fail()
	}
}

func TestSearchClient_FindUsersTimeoutError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(BrokenServer))
	defer ts.Close()

	sc := &SearchClient{
		URL: ts.URL,
	}

	_, err := sc.FindUsers(SearchRequest{})
	if err == nil {
		t.Fail()
	}
}

type ErrorTestCase struct {
	Request     SearchRequest
	AccessToken string
	Error       string
}

var errorTestCases = []ErrorTestCase{
	ErrorTestCase{
		AccessToken: "No-Token",
		Error:       "Bad AccessToken",
	},
	ErrorTestCase{
		Error: "Bad AccessToken",
	},
	ErrorTestCase{
		Request: SearchRequest{
			Limit:  10,
			Offset: 100,
		},
		AccessToken: "FindUsers-Test-Token",
		Error:       "SearchServer fatal error",
	},
	ErrorTestCase{
		Request: SearchRequest{
			Limit:   10,
			OrderBy: 2,
		},
		AccessToken: "FindUsers-Test-Token",
		Error:       "",
	},
	ErrorTestCase{
		Request: SearchRequest{
			Limit:      10,
			OrderField: "about",
		},
		AccessToken: "FindUsers-Test-Token",
		Error:       "OrderFeld about invalid",
	},
	ErrorTestCase{
		Request: SearchRequest{
			Limit:      10,
			OrderField: "email",
		},
		AccessToken: "FindUsers-Test-Token",
		Error:       "unknown bad request error: Unknown error",
	},
}

func TestSearchClient_FindUsersErrors(t *testing.T) {
	var fail = false

	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()

	for _, test := range errorTestCases {
		sc := &SearchClient{
			AccessToken: test.AccessToken,
			URL:         ts.URL,
		}

		_, err := sc.FindUsers(test.Request)
		if err == nil {
			t.Errorf("Error test failed. Expected error %q.", test.Error)
			fail = true
			continue
		}

		if test.Error == "" {
			continue
		} else if err.Error() != test.Error {
			t.Errorf("Error test failed. Got error %q. Expected error %q.", test.Error, err.Error())
			fail = true
		}
	}

	if fail {
		t.Fail()
	}
}

func queryProcess(users []User, query string) []User {
	res := make([]User, 0, len(users))
	for _, u := range users {
		if strings.Contains(strings.ToLower(u.Name), query) || strings.Contains(strings.ToLower(u.About), query) {
			res = append(res, u)
		}
	}
	return res
}

type QueryTestCase struct {
	Request  SearchRequest
	Response SearchResponse
	IsError  bool
}

var queryTestCases = []QueryTestCase{
	QueryTestCase{
		Request: SearchRequest{
			Query: "cillum cupidatat sit",
			Limit: 25,
		},
		Response: SearchResponse{
			Users: []User{
				User{
					Name: "Glenn Jordan",
				},
			},
		},
		IsError: false,
	},
}

func TestSearchClient_FindUsersQuery(t *testing.T) {
	var fail = false
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()

	sc := &SearchClient{
		AccessToken: "FindUsers-Test-Token",
		URL:         ts.URL,
	}

	for _, qtc := range queryTestCases {
		srs, err := sc.FindUsers(qtc.Request)
		if err != nil {
			if !qtc.IsError {
				fail = true
				t.Errorf("Query test failed. Unexpected error: %q", err)
			}
			continue
		}

		if len(srs.Users) == 0 {
			t.Errorf("Query test failed. User not found.")
			fail = true
			continue
		}

		if srs.Users[0].Name != qtc.Response.Users[0].Name {
			t.Errorf("Query test failed. Got: %q. Expected: %q", srs.Users[0].Name, qtc.Response.Users[0].Name)
			fail = true
			continue
		}
	}

	if fail {
		t.Fail()
	}
}

type PaginationTestCase struct {
	Request      SearchRequest
	CountRecords int
	IsError      bool
}

var paginationTestCases = []PaginationTestCase{
	PaginationTestCase{
		Request: SearchRequest{
			Limit: -5,
		},
		IsError: true,
	},
	PaginationTestCase{
		Request: SearchRequest{
			Limit: 10,
		},
		CountRecords: 10,
		IsError:      false,
	},
	PaginationTestCase{
		Request: SearchRequest{
			Limit: 30,
		},
		CountRecords: 25,
		IsError:      false,
	},
	PaginationTestCase{
		Request: SearchRequest{
			Offset: -5,
		},
		IsError: true,
	},
	PaginationTestCase{
		Request: SearchRequest{
			Limit:  10,
			Offset: 5,
		},
		CountRecords: 10,
		IsError:      false,
	},
	PaginationTestCase{
		Request: SearchRequest{
			Limit: 0,
		},
		IsError: true,
	},
}

func TestSearchClient_FindUsersPagination(t *testing.T) {
	var fail = false
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()

	sc := &SearchClient{
		AccessToken: "FindUsers-Test-Token",
		URL:         ts.URL,
	}

	for _, ptc := range paginationTestCases {
		srs, err := sc.FindUsers(ptc.Request)
		if err != nil {
			if !ptc.IsError {
				fail = true
				t.Errorf("Limit test failed. Unexpected error: %q", err)
			}
			continue
		}

		expected := ptc.CountRecords
		result := len(srs.Users)
		if result != expected {
			t.Errorf("Limit test failed. Got records: %d. Expected records %d.", result, expected)
			fail = true
			continue
		}
	}

	if fail {
		t.Fail()
	}
}
