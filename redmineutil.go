package redmineutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

var usersCollection RedmineUsersCollection

// HandleMessage handles a message from the SSI bot
func HandleMessage(msg string, userFirstName string, key string) (resp string) {
	if strings.Contains(msg, "get") || strings.Contains(msg, "show") {
		issues, err := getIssues(key, userFirstName, -1)
		if err != nil {
			return fmt.Sprintf("Welllll crud, we hit a snag: %s", err.Error())
		}

		resp := fmt.Sprintf("I found %d recent issues assigned to you, %s:\n", len(issues), userFirstName)
		for _, issue := range issues {
			resp += fmt.Sprintf("<https://vault.softwaresysinc.net/redmine/issues/%d|Issue #%d> - %s \n", issue.ID, issue.ID, issue.Subject)
		}

		return resp
	}

	return fmt.Sprintf("Hi %s, I didn't understand your instructions", userFirstName)
}

func getIssues(key string, user string, limit int) (ret []Issue, retErr error) {
	log.Printf("redmineutil.GetIssues - Getting issues...")
	client := &http.Client{}

	if usersCollection.TotalCount == int64(0) {
		log.Printf("redmineutil.GetIssues -   Getting users first...")
		getUsers(client, key)
		log.Printf("redmineutil.GetIssues -   Got %d user records...", len(usersCollection.Users))
	}

	// find the ID for the user we're getting issues for
	userId := int64(-1)
	for _, usr := range usersCollection.Users {
		if usr.Firstname == user || usr.Lastname == user {
			userId = usr.ID
			break
		}
	}

	if userId == -1 {
		et := fmt.Sprintf("redmineutil.GetIssues -   Couldn't find a user record for \"%s\".", user)
		log.Printf(et)
		return nil, errors.New(et)
	}

	minDate := time.Now().AddDate(-2, 0, 0).Format("2006-01-02")
	issuesUrl := fmt.Sprintf("https://vault.softwaresysinc.net/redmine/issues.json?assigned_to_id=%d&created_on=%3E%3D%s", userId, minDate)

	req, err := http.NewRequest("GET", issuesUrl, nil)
	if err != nil {
		et := fmt.Sprintf("redmineutil.GetIssues failed to create a request to get issues: %s", err.Error())
		log.Printf(et)
		return nil, errors.New(et)
	}

	req.Header.Add("User-Agent", "SSIbot/0.1")
	req.Header.Add("X-Redmine-API-Key", key)

	resp, err := client.Do(req)
	if err != nil {
		et := fmt.Sprintf("redmineutil.GetIssues failed to get issues response: %s", err.Error())
		log.Printf(et)
		return nil, errors.New(et)
	}

	var issuesCollection RedmineIssuesCollection
	json.NewDecoder(resp.Body).Decode(&issuesCollection)

	log.Printf("redmineutil.GetIssues -   Got %d issue records...", len(issuesCollection.Issues))

	return issuesCollection.Issues, nil
}

func getUsers(client *http.Client, key string) {
	req, err := http.NewRequest("GET", "https://vault.softwaresysinc.net/redmine/users.json", nil)
	if err != nil {
		et := fmt.Sprintf("redmineutil.GetIssues failed to create a request to get users: %s", err.Error())
		log.Printf(et)
		//return nil, errors.New(et)
		return
	}

	req.Header.Add("User-Agent", "SSIbot/0.1")
	req.Header.Add("X-Redmine-API-Key", key)

	resp, err := client.Do(req)
	if err != nil {
		et := fmt.Sprintf("redmineutil.GetIssues failed to get users response: %s", err.Error())
		log.Printf(et)
		//return nil, errors.New(et)
		return
	}

	json.NewDecoder(resp.Body).Decode(&usersCollection)
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

// RedmineIssuesCollection struct containing issues
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
