package redmineapi

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

var redmineAPIKey string
var redmineBaseURL string
var usersCollection RedmineUsersCollection

// Client API client object
type Client struct{}

// InitializeNewClient takes a couple needed settings, so they don't need to be passed repeatedly.
func InitializeNewClient(apiKey string, redmineURL string) (rc *Client) {
	redmineAPIKey = apiKey
	redmineBaseURL = redmineURL

	return &Client{}
}

// HandleMessage handles a message from the SSIbot
func (rc Client) HandleMessage(msg string, userFirstName string) (resp string) {
	if redmineAPIKey == "" || redmineBaseURL == "" {
		return "RedmineApiClient has not been initialized."
	}

	if len(usersCollection.Users) == 0 {
		err := getUsers()
		if err != nil || usersCollection.TotalCount == 0 {
			return "Unable to get users from Redmine."
		}
	}

	if strings.Contains(msg, "get") || strings.Contains(msg, "show") {
		issues, err := getIssues(userFirstName)
		if err != nil {
			return fmt.Sprintf("Well crud, we hit a snag: %s", err.Error())
		}

		resp := fmt.Sprintf("I found %d open issues assigned to you, %s:\n", len(issues), userFirstName)
		for _, issue := range issues {
			resp += fmt.Sprintf(
				"%s <%s/issues/%d|Issue #%d> - %s\n",
				issue.Project.Name,
				redmineBaseURL,
				issue.ID,
				issue.ID,
				issue.Subject)
		}

		return resp
	} else if strings.Contains(msg, "close") || strings.Contains(msg, "reject") {
		return setIssueClosed(msg, userFirstName)
	} else if strings.Contains(msg, "ready to test") {
		return setIssueReadyToTest(msg, userFirstName)
	}

	return fmt.Sprintf("Hi %s, I didn't understand your instructions", userFirstName)
}

func getIssues(user string) (ret []RedmineIssue, retErr error) {
	log.Printf("getIssues - Getting ze issues...")
	client := &http.Client{}

	// find the ID for the user we're getting issues for
	userID := int64(-1)
	for _, usr := range usersCollection.Users {
		if usr.Firstname == user || usr.Lastname == user {
			userID = usr.ID
			break
		}
	}

	if userID == -1 {
		et := fmt.Sprintf("getIssues -   Couldn't find a user record for \"%s\".", user)
		log.Printf(et)
		return nil, errors.New(et)
	}

	minDate := url.QueryEscape(">=" + time.Now().AddDate(-2, 0, 0).Format("2006-01-02"))
	issuesURL := fmt.Sprintf("%s/issues.json?assigned_to_id=%d&created_on=%s", redmineBaseURL, userID, minDate)

	req, err := http.NewRequest("GET", issuesURL, nil)
	if err != nil {
		et := fmt.Sprintf("GetIssues failed to create a request to get issues: %s", err.Error())
		log.Printf(et)
		return nil, errors.New(et)
	}

	req.Header.Add("User-Agent", "go-redmineapi/0.1")
	req.Header.Add("X-Redmine-API-Key", redmineAPIKey)

	resp, err := client.Do(req)
	if err != nil {
		et := fmt.Sprintf("GetIssues failed to get issues response: %s", err.Error())
		log.Printf(et)
		return nil, errors.New(et)
	}

	var issuesCollection RedmineIssuesCollection
	json.NewDecoder(resp.Body).Decode(&issuesCollection)

	log.Printf("GetIssues -   Got %d issue records...", len(issuesCollection.Issues))

	return issuesCollection.Issues, nil
}

func setIssueClosed(msg string, userFirstName string) string {
	log.Printf("setIssueClosed - Attempting to close an issue...")

	issID := -1
	for _, token := range strings.Split(msg, " ") {
		testID, err := strconv.Atoi(token)
		if err == nil {
			issID = testID
			break
		}
	}

	if issID == -1 {
		log.Printf("setIssueClosed -   Unable to determine issue ID from \"%s\"", msg)
		return fmt.Sprintf("I couldn't close an issue from \"%s\", because I couldn't figure out what the issue ID was.", msg)
	}

	log.Printf("setIssueClosed -   Issue with ID #%d is to be set to closed.", issID)

	delURL := fmt.Sprintf("%s/issues/%d.json", redmineBaseURL, issID)
	rawJSON := []byte(fmt.Sprintf("{ \"issue\": { \"status_id\": \"5\", \"notes\": \"Closed by SSIbot on behalf of %s.\" }}", userFirstName))

	req, err := http.NewRequest("PUT", delURL, bytes.NewBuffer(rawJSON))
	if err != nil {
		log.Printf("setIssueClosed -   Failed creating the PUT request: %s", err.Error())
		return fmt.Sprintf("I couldn't close an issue, becuse something went wrong ")
	}

	req.Header.Add("User-Agent", "sb-go-redmineapi/0.1")
	req.Header.Add("X-Redmine-API-Key", redmineAPIKey)
	req.Header.Add("Content-Type", "application/json")
	req.ContentLength = int64(len(rawJSON))

	client := &http.Client{}
	_, err = client.Do(req)
	if err != nil {
		log.Printf("setIssueClosed -   Failed while trying to get a response for the PUT request: %s", err.Error())
		return fmt.Sprintf("I didn't get a response from Redmine when closing issue #%d, %s", issID, err.Error())
	}

	log.Printf("setIssueClosed -   Successfully set issue #%d to closed.", issID)

	return fmt.Sprintf(
		"Alrighty, I've closed <%s/issues/%d|Issue #%d>.",
		redmineBaseURL,
		issID,
		issID)
}

func setIssueReadyToTest(msg string, userFirstName string) string {
	log.Printf("setIssueReadyToTest - Attempting to mark an issue as ready to test")

	issID := -1
	assigneeID := -1
	foundAssign := false

	tokens := strings.Split(msg, " ")
	for _, token := range tokens {
		if token == "assign" {
			foundAssign = true
			continue
		}
		if foundAssign {
			for _, usr := range usersCollection.Users {
				if strings.ToUpper(usr.Firstname) == strings.ToUpper(token) || strings.ToUpper(usr.Lastname) == strings.ToUpper(token) {
					assigneeID = int(usr.ID)
					continue
				}
			}
		}

		testID, err := strconv.Atoi(token)
		if err == nil {
			issID = testID
		}
	}

	if issID == -1 {
		log.Printf("setIssueReadyToTest -   Unable to determine issue ID from \"%s\"", msg)
		return "I couldn't figure out what the issue ID was, so I had to give up."
	}
	if assigneeID == -1 {
		log.Printf("setIssueReadyToTest -   Unable to determine new assignee ID from \"%s\"", msg)
		return "I couldn't figure out who to assign the issue to, so I had to give up."
	}

	log.Printf("setIssueReadyToTest -   Marking Issue #%d as ready to test, and assigning it to user ID %d", issID, assigneeID)

	updURL := fmt.Sprintf("%s/issues/%d.json", redmineBaseURL, issID)
	rawJSON := []byte(fmt.Sprintf(
		"{ \"issue\": { \"status_id\": \"3\", \"assigned_to_id\": \"%d\", \"notes\": \"Marked Ready to Test by SSIbot on behalf of %s.\" }}",
		assigneeID, userFirstName))

	req, err := http.NewRequest("PUT", updURL, bytes.NewBuffer(rawJSON))
	if err != nil {
		log.Printf("setIssueReadyToTest -   Failed while creating the PUT request: %s", err.Error())
		return fmt.Sprintf("I failed while creating the PUT request to update the issue: %s", err.Error())
	}

	req.Header.Add("User-Agent", "go-redmineapi/0.1")
	req.Header.Add("X-Redmine-API-Key", redmineAPIKey)
	req.Header.Add("Content-Type", "application/json")
	req.ContentLength = int64(len(rawJSON))

	client := &http.Client{}
	_, err = client.Do(req)
	if err != nil {
		return fmt.Sprintf("I failed while trying to get a response: %s", err.Error())
	}

	log.Printf("setIssueReadyToTest -   Marked issue %d as ready to test.", issID)
	return fmt.Sprintf(
		"Alright, I've marked <%s/issues/%d|Issue #%d> as ready to test.",
		redmineBaseURL,
		issID,
		issID)
}

func getUsers() error {
	log.Printf("getUsers - Attempting to close an issue")

	client := &http.Client{}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/users.json", redmineBaseURL), nil)
	if err != nil {
		et := fmt.Sprintf("getUsers -   failed to create request: %s", err.Error())
		log.Printf(et)
		return errors.New(et)
	}

	req.Header.Add("User-Agent", "go-redmineapi/0.1")
	req.Header.Add("X-Redmine-API-Key", redmineAPIKey)

	resp, err := client.Do(req)
	if err != nil {
		et := fmt.Sprintf("getUsers -   failed to get response: %s", err.Error())
		log.Printf(et)
		return errors.New(et)
	}

	json.NewDecoder(resp.Body).Decode(&usersCollection)

	return nil
}

// RedmineUsersCollection represents a collection of RedmineUser objects
type RedmineUsersCollection struct {
	Users      []RedmineUser `json:"users"`
	TotalCount int64         `json:"total_count"`
	Offset     int64         `json:"offset"`
	Limit      int64         `json:"limit"`
}

// RedmineUser represents a user in the Redmine system
type RedmineUser struct {
	ID          int64  `json:"id"`
	Login       string `json:"login"`
	Firstname   string `json:"firstname"`
	Lastname    string `json:"lastname"`
	Mail        string `json:"mail"`
	CreatedOn   string `json:"created_on"`
	LastLoginOn string `json:"last_login_on"`
}

// RedmineIssuesCollection represents a collection of RedmineIssue objects
type RedmineIssuesCollection struct {
	Issues     []RedmineIssue `json:"issues"`
	TotalCount int64          `json:"total_count"`
	Offset     int64          `json:"offset"`
	Limit      int64          `json:"limit"`
}

// RedmineIssue represents an issue in the Redmine system
type RedmineIssue struct {
	ID           int64                `json:"id"`
	Project      RedmineProperty      `json:"project"`
	Tracker      RedmineProperty      `json:"tracker"`
	Status       RedmineProperty      `json:"status"`
	Priority     RedmineProperty      `json:"priority"`
	Author       RedmineProperty      `json:"author"`
	AssignedTo   RedmineProperty      `json:"assigned_to"`
	Subject      string               `json:"subject"`
	Description  string               `json:"description"`
	StartDate    string               `json:"start_date"`
	DoneRatio    int64                `json:"done_ratio"`
	CreatedOn    string               `json:"created_on"`
	UpdatedOn    string               `json:"updated_on"`
	Category     RedmineProperty      `json:"category"`
	CustomFields []RedmineCustomField `json:"custom_fields"`
	FixedVersion RedmineProperty      `json:"fixed_version"`
	DueDate      string               `json:"due_date"`
}

// RedmineProperty represents a property field in Redmine data
type RedmineProperty struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// RedmineCustomField represents a custom field in Redmine
type RedmineCustomField struct {
	ID    int64                  `json:"id"`
	Name  RedmineCustomFieldName `json:"name"`
	Value string                 `json:"value"`
}

// RedmineCustomFieldName represents a custom field's name
type RedmineCustomFieldName string

const (
	// CallerOrContactName The caller's or contact's name.
	CallerOrContactName RedmineCustomFieldName = "Caller or Contact Name"
	// CustomWork Flag indicating whether this is regarding custom work for the client.
	CustomWork RedmineCustomFieldName = "Custom Work"
	// Customer SSI Customer name.
	Customer RedmineCustomFieldName = "Customer"
	// DBName The DB name associated with this issue (usually SSI internal).
	DBName RedmineCustomFieldName = "DB Name"
	// ECLocation The name of the extra-curricular location associated with this issue.
	ECLocation RedmineCustomFieldName = "EC Location"
	// EmpNo The employee number associated with this issue.
	EmpNo RedmineCustomFieldName = "Emp No"
	// Filename The attachment filename associated with this issue.
	Filename RedmineCustomFieldName = "Filename"
	// ProgramName The name of the Progress procedure associated with this issue.
	ProgramName RedmineCustomFieldName = "Program Name"
	// Received The way in which this issue was reported.
	Received RedmineCustomFieldName = "Received"
)
