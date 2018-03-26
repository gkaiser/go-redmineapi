package redmineutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
)

var usersCollection RedmineUsersCollection

// GetIssues returns issues for a given Redmine user
func GetIssues(key string, user string, limit int) ([]Issue, error) {
	log.Printf("redmineutil.GetIssues - Getting issues...")

	if usersCollection.TotalCount == int64(0) {
		log.Printf("redmineutil.GetIssues -   Getting users first...")

		client := &http.Client{}

		req, err := http.NewRequest("GET", "https://vault.softwaresysinc.net/redmine/users.json", nil)
		if err != nil {
			et := fmt.Sprintf("redmineutil.GetIssues failed to create a request to get users: %s", err.Error())
			log.Printf(et)
			return nil, errors.New(et)
		}

		req.Header.Add("User-Agent", "SSIbot/0.1")
		req.Header.Add("X-Redmine-API-Key", key)

		resp, err := client.Do(req)
		if err != nil {
			et := fmt.Sprintf("redmineutil.GetIssues failed to get users response: %s", err.Error())
			log.Printf(et)
			return nil, errors.New(et)
		}

		json.NewDecoder(resp.Body).Decode(&usersCollection)

		log.Printf("redmineutil.GetIssues -   Got %d user records...", len(usersCollection.Users))
	}

	return make([]Issue, 0), nil
}

type RedmineUsersCollection struct {
	Users      []RedmineUser `json:"users"`
	TotalCount int64         `json:"total_count"`
	Offset     int64         `json:"offset"`
	Limit      int64         `json:"limit"`
}

type RedmineUser struct {
	ID          int64  `json:"id"`
	Login       string `json:"login"`
	Firstname   string `json:"firstname"`
	Lastname    string `json:"lastname"`
	Mail        string `json:"mail"`
	CreatedOn   string `json:"created_on"`
	LastLoginOn string `json:"last_login_on"`
}

// RedmineIssues struct containing issues
type RedmineIssuesCollection struct {
	Issues     []Issue `json:"issues"`
	TotalCount int64   `json:"total_count"`
	Offset     int64   `json:"offset"`
	Limit      int64   `json:"limit"`
}

type Issue struct {
	ID           int64           `json:"id"`
	Project      RedmineProperty `json:"project"`
	Tracker      RedmineProperty `json:"tracker"`
	Status       RedmineProperty `json:"status"`
	Priority     RedmineProperty `json:"priority"`
	Author       RedmineProperty `json:"author"`
	AssignedTo   RedmineProperty `json:"assigned_to"`
	Subject      string          `json:"subject"`
	Description  string          `json:"description"`
	StartDate    string          `json:"start_date"`
	DoneRatio    int64           `json:"done_ratio"`
	CreatedOn    string          `json:"created_on"`
	UpdatedOn    string          `json:"updated_on"`
	Category     RedmineProperty `json:"category"`
	CustomFields []CustomField   `json:"custom_fields"`
	FixedVersion RedmineProperty `json:"fixed_version"`
	DueDate      string          `json:"due_date"`
}

type RedmineProperty struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type CustomField struct {
	ID    int64  `json:"id"`
	Name  Name   `json:"name"`
	Value string `json:"value"`
}

type Name string

const (
	CallerOrContactName Name = "Caller or Contact Name"
	CustomWork          Name = "Custom Work"
	Customer            Name = "Customer"
	DBName              Name = "DB Name"
	ECLocation          Name = "EC Location"
	EmpNo               Name = "Emp No"
	Filename            Name = "Filename"
	ProgramName         Name = "Program Name"
	Received            Name = "Received"
)
