package redmineutil

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// mysql> select * from issue_statuses;
// +----+---------------+-----------+----------+--------------------+
// | id | name          | is_closed | position | default_done_ratio |
// +----+---------------+-----------+----------+--------------------+
// |  1 | New           |         0 |        1 |               NULL |
// |  2 | Assigned      |         0 |        2 |               NULL |
// |  3 | Ready to Test |         0 |        4 |               NULL |
// |  5 | Closed        |         1 |        5 |               NULL |
// |  6 | Rejected      |         1 |        6 |               NULL |
// |  7 | Feedback      |         0 |        3 |               NULL |
// +----+---------------+-----------+----------+--------------------+

const BaseRedmineUrl = "https://vault.softwaresysinc.net/redmine"

var usersCollection RedmineUsersCollection

// HandleMessage handles a message from the SSI bot
func HandleMessage(msg string, userFirstName string, key string) (resp string) {
	if len(usersCollection.Users) == 0 {
		log.Printf("redmineutil.GetIssues -   Getting users first...")
		getUsers(key)
		log.Printf("redmineutil.GetIssues -   Got %d user records...", len(usersCollection.Users))
	}

	if strings.Contains(msg, "get") || strings.Contains(msg, "show") {
		issues, err := getIssues(key, userFirstName, -1)
		if err != nil {
			return fmt.Sprintf("Well crud, we hit a snag: %s", err.Error())
		}

		resp := fmt.Sprintf("I found %d open issues assigned to you, %s:\n", len(issues), userFirstName)
		for _, issue := range issues {
			resp += fmt.Sprintf(
				"%s <%s/issues/%d|Issue #%d> - %s\n",
				issue.Project.Name,
				BaseRedmineUrl,
				issue.ID,
				issue.ID,
				issue.Subject)
		}

		return resp
	} else if strings.Contains(msg, "close") || strings.Contains(msg, "reject") {
		return closeIssue(msg, userFirstName, key)
	} else if strings.Contains(msg, "ready to test") {
		return setReadyToTestStatus(msg, userFirstName, key)
	}

	return fmt.Sprintf("Hi %s, I didn't understand your instructions", userFirstName)
}

func getIssues(key string, user string, limit int) (ret []Issue, retErr error) {
	log.Printf("redmineutil.GetIssues - Getting issues...")
	client := &http.Client{}

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

	minDate := url.QueryEscape(">=" + time.Now().AddDate(-2, 0, 0).Format("2006-01-02"))
	issuesUrl := fmt.Sprintf("%s/issues.json?assigned_to_id=%d&created_on=%s", BaseRedmineUrl, userId, minDate)

	log.Printf("redmineutil.GetIssues -    Redmine URL: %s", issuesUrl)

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

func closeIssue(msg string, userFirstName string, key string) string {
	log.Printf("redmineutil.closeIssue - Attempting to close an issue")

	issId := -1
	for _, token := range strings.Split(msg, " ") {
		testId, err := strconv.Atoi(token)
		if err == nil {
			issId = testId
			break
		}
	}

	if issId == -1 {
		log.Printf("redmineutil.closeIssue - Unable to determine issue ID from \"%s\"", msg)
		return "I couldn't figure out what the issue ID was, so I had to give up."
	}

	log.Printf("redmineutil.closeIssue -   Issue #%d is to be closed", issId)

	delUrl := fmt.Sprintf("%s/issues/%d.json", BaseRedmineUrl, issId)
	rawJson := []byte(fmt.Sprintf("{ \"issue\": { \"status_id\": \"5\", \"notes\": \"Closed by SSI bot on behalf of %s.\" }}", userFirstName))

	req, err := http.NewRequest("PUT", delUrl, bytes.NewBuffer(rawJson))
	if err != nil {
		log.Printf("redmineutil.closeIssue -   Failed while creating the PUT request: %s", err.Error())
		return fmt.Sprintf("I failed while creating the PUT request to update the issue: %s", err.Error())
	}

	req.Header.Add("User-Agent", "SSIbot/0.1")
	req.Header.Add("X-Redmine-API-Key", key)
	req.Header.Add("Content-Type", "application/json")
	req.ContentLength = int64(len(rawJson))

	client := &http.Client{}
	_, err = client.Do(req)
	if err != nil {
		return fmt.Sprintf("I failed while trying to get a response: %s", err.Error())
	}

	log.Printf("redmineutil.closeIssue -   Closed issue %d successfully.", issId)
	return fmt.Sprintf("Alright, I've closed Issue #%d.", issId)
}

func setReadyToTestStatus(msg string, userFirstName string, key string) string {
	log.Printf("redmineutil.setReadyToTestStatus - Attempting to mark an issue as ready to test")

	issId := -1
	assigneeId := -1
	foundAssign := false

	tokens := strings.Split(msg, " ")
	for _, token := range tokens {
		if token == "assign" {
			foundAssign = true
			continue
		}
		if foundAssign {
			for _, usr := range usersCollection.Users {
				if strings.ToUpper(usr.Firstname) == strings.ToUpper(userFirstName) {
					assigneeId = int(usr.ID)
					continue
				}
			}
		}

		testId, err := strconv.Atoi(token)
		if err == nil {
			issId = testId
			continue
		}
	}

	if issId == -1 {
		log.Printf("redmineutil.setReadyToTestStatus -   Unable to determine issue ID from \"%s\"", msg)
		return "I couldn't figure out what the issue ID was, so I had to give up."
	}
	if assigneeId == -1 {
		log.Printf("redmineutil.setReadyToTestStatus -   Unable to determine new assignee ID from \"%s\"", msg)
		return "I couldn't figure out who to assign the issue to, so I had to give up."
	}

	log.Printf("redmineutil.setReadyToTestStatus -   Marking Issue #%d as ready to test, and assigning it to user ID %d", issId, assigneeId)

	updUrl := fmt.Sprintf("%s/issues/%d.json", BaseRedmineUrl, issId)
	rawJson := []byte(fmt.Sprintf(
		"{ \"issue\": { \"status_id\": \"3\", \"assigned_to_id\": %d, \"notes\": \"Marked Ready to Test by SSI bot on behalf of %s.\" }}",
		assigneeId, userFirstName))

	req, err := http.NewRequest("PUT", updUrl, bytes.NewBuffer(rawJson))
	if err != nil {
		log.Printf("redmineutil.setReadyToTestStatus -   Failed while creating the PUT request: %s", err.Error())
		return fmt.Sprintf("I failed while creating the PUT request to update the issue: %s", err.Error())
	}

	log.Printf("redmineutil.setReadyToTestStatus -   Writing \"%s\" to %s", string(rawJson), updUrl)

	req.Header.Add("User-Agent", "SSIbot/0.1")
	req.Header.Add("X-Redmine-API-Key", key)
	req.Header.Add("Content-Type", "application/json")
	req.ContentLength = int64(len(rawJson))

	client := &http.Client{}
	_, err = client.Do(req)
	if err != nil {
		return fmt.Sprintf("I failed while trying to get a response: %s", err.Error())
	}

	log.Printf("redmineutil.setReadyToTestStatus -   Marked issue %d as ready to test.", issId)
	return fmt.Sprintf("Alright, I've marked Issue #%d as ready to test.", issId)
}

func getUsers(key string) {
	client := &http.Client{}

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
	ID    int64           `json:"id"`
	Name  CustomFieldName `json:"name"`
	Value string          `json:"value"`
}

type CustomFieldName string

const (
	CallerOrContactName CustomFieldName = "Caller or Contact Name"
	CustomWork          CustomFieldName = "Custom Work"
	Customer            CustomFieldName = "Customer"
	DBName              CustomFieldName = "DB Name"
	ECLocation          CustomFieldName = "EC Location"
	EmpNo               CustomFieldName = "Emp No"
	Filename            CustomFieldName = "Filename"
	ProgramName         CustomFieldName = "Program Name"
	Received            CustomFieldName = "Received"
)
