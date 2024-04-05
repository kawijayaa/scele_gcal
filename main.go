package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-co-op/gocron/v2"
	"github.com/gocolly/colly"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	tasksapi "google.golang.org/api/tasks/v1"
)

type SceleClient struct {
	Username          string
	Password          string
	Excluded_Courses  []int
	Excluded_Keywords []string
	Client            *http.Client
	Cookie            []*http.Cookie
}

type SceleConfig struct {
	Username          string   `json:"username"`
	Password          string   `json:"password"`
	Excluded_Courses  []int    `json:"excluded_courses"`
	Excluded_Keywords []string `json:"excluded_keywords"`
}

type SceleLoginForm struct {
	username   string
	password   string
	logintoken string
}

type Task struct {
	Name     string
	Course   string
	Deadline time.Time
	URL      string
	Excluded bool
}

func NewSceleClient(scele_config SceleConfig) *SceleClient {
	jar, _ := cookiejar.New(nil)
	return &SceleClient{
		scele_config.Username,
		scele_config.Password,
		scele_config.Excluded_Courses,
		scele_config.Excluded_Keywords,
		&http.Client{Jar: jar},
		[]*http.Cookie{},
	}
}

func (sc *SceleClient) NewCollector() *colly.Collector {
	c := colly.NewCollector()
	c.SetCookies("https://scele.cs.ui.ac.id/", sc.Cookie)
	return c
}

func (sc *SceleClient) Login() bool {
	log.Println("Logging in to SCeLe...")
	response, err := sc.Client.Get("https://scele.cs.ui.ac.id/login/index.php")
	if err != nil {
		log.Fatalln(err)
		return false
	}

	document, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		log.Fatalln(err)
		return false
	}

	var token string
	document.Find("input").Map(func(i int, s *goquery.Selection) string {
		name, _ := s.Attr("name")
		if name == "logintoken" {
			token, _ = s.Attr("value")
		}
		return ""
	})

	response, err = sc.Client.PostForm("https://scele.cs.ui.ac.id/login/index.php",
		map[string][]string{
			"username":   {sc.Username},
			"password":   {sc.Password},
			"anchor":     {""},
			"logintoken": {token},
		})
	if err != nil {
		log.Fatalln(err.Error())
		return false
	}

	url, _ := url.Parse("https://scele.cs.ui.ac.id/")
	if sc.Client.Jar.Cookies(url)[0].Name == "MoodleSession" {
		sc.Cookie = sc.Client.Jar.Cookies(url)
		return true
	}

	return false
}

func (sc *SceleClient) GetTask(event_url string) Task {
	c := sc.NewCollector()
	re := regexp.MustCompile(`(?P<event_id>#.*)$`)
	matches := re.FindStringSubmatch(event_url)
	event_id := matches[re.SubexpIndex("event_id")]
	log.Printf("Fetching details from event_id: %v\n", event_id)
	var task Task

	c.OnHTML(event_id, func(h *colly.HTMLElement) {
		var task_name string
		var task_url string
		var task_course string
		var task_excluded bool = true

		h.DOM.Find("a[href]").Map(func(i int, s *goquery.Selection) string {
			href, _ := s.Attr("href")
			if strings.Contains(href, "assign") && !strings.Contains(href, "&action=") {
				task_name = s.Text()
				task_url = href
			}

			if strings.Contains(href, "course") && !strings.Contains(href, "update") {
				re := regexp.MustCompile(`\?id=(?P<course_id>\d*)$`)
				matches := re.FindStringSubmatch(href)
				course_id, _ := strconv.Atoi(matches[re.SubexpIndex("course_id")])

				if !slices.Contains(sc.Excluded_Courses, course_id) {
					task_course = s.Text()
					task_excluded = false
				}
			}

			return ""
		})

		re := regexp.MustCompile(`&time=(?P<epoch>\d*)`)
		matches := re.FindStringSubmatch(event_url)
		epoch_time, _ := strconv.Atoi(matches[re.SubexpIndex("epoch")])
		loc, _ := time.LoadLocation("Asia/Bangkok")

		task = Task{
			Name:     task_name,
			URL:      task_url,
			Course:   task_course,
			Deadline: time.Unix(int64(epoch_time), 0).In(loc),
			Excluded: task_excluded,
		}
	})

	c.Visit(event_url)
	return task
}

func (sc *SceleClient) FetchCurrentTasks() []Task {
	log.Println("Fetching tasks from SCeLE...")
	c := sc.NewCollector()
	tasks := make([]Task, 0)
	c.OnHTML("li", func(h *colly.HTMLElement) {
		h.DOM.Children().Map(func(i int, s *goquery.Selection) string {
			href, _ := s.Attr("href")
			if strings.Contains(href, "&time=") {
				event := s.Nodes[0]
				for _, keyword := range sc.Excluded_Keywords {
					if strings.Contains(event.FirstChild.Data, keyword) {
						return ""
					}
				}

				task := sc.GetTask(event.Attr[0].Val)

				if task.Deadline.Before(time.Now()) {
					return ""
				}

				if task.Excluded {
					return ""
				}

				tasks = append(tasks, task)
			}
			return ""
		})
	})
	c.Visit("https://scele.cs.ui.ac.id/calendar/view.php?view=month")
	return tasks
}

func getClient(config *oauth2.Config) *http.Client {
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func saveToken(path string, token *oauth2.Token) {
	log.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func main() {
	s, _ := gocron.NewScheduler()

	t := gocron.NewTask(func() {
		log.Println("Starting fetch...")
		ctx := context.Background()
		b, err := os.ReadFile("config.json")
		if err != nil {
			log.Fatalf("Unable to read client secret file: %v", err)
		}

		config, err := google.ConfigFromJSON(b, tasksapi.TasksScope, tasksapi.TasksReadonlyScope)
		if err != nil {
			log.Fatalf("Unable to parse client secret file to config: %v", err)
		}
		client := getClient(config)

		api, err := tasksapi.NewService(ctx, option.WithHTTPClient(client))
		if err != nil {
			log.Fatalf("Unable to retrieve tasks Client %v", err)
		}

		tasklist, _ := api.Tasklists.List().MaxResults(1).Do()
		fetched_tasks, _ := api.Tasks.List(tasklist.Items[0].Id).Do(googleapi.QueryParameter("showHidden", "true"))
		var existing_tasks []string

		for _, task := range fetched_tasks.Items {
			existing_tasks = append(existing_tasks, task.Notes)
		}

		scele_config_file, err := os.Open("scele_config.json")
		if err != nil {
			log.Fatalln("SCeLe config not found!")
		}

		var scele_config SceleConfig
		json.NewDecoder(scele_config_file).Decode(&scele_config)

		sc := NewSceleClient(scele_config)
		ok := sc.Login()
		if !ok {
			log.Fatalln("Cannot login!")
		}

		log.Println("Login to SCeLe successful!")

		tasks := sc.FetchCurrentTasks()

		for _, task := range tasks {
			if slices.Contains(existing_tasks, task.Course+"\n"+task.URL) {
				log.Printf("Task \"%v\" already exists, skipping...\n", task.Name)
				continue
			}

			task_obj := tasksapi.Task{
				Title: task.Name,
				Due:   task.Deadline.AddDate(0, 0, 1).Format(time.RFC3339),
				Notes: task.Course + "\n" + task.URL,
			}
			_, err = api.Tasks.Insert(tasklist.Items[0].Id, &task_obj).Do()
			if err != nil {
				log.Fatalln(err)
			}
			log.Printf("Created task \"%v\"\n", task.Name)
		}
	})

	s.NewJob(
		gocron.DurationJob(24*time.Hour),
		t,
	)

	s.Start()

	select {
	case <-time.After(time.Minute):
	}

	s.Shutdown()
}
