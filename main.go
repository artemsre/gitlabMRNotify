package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang/gddo/httputil/header"
)

var MrId map[string]int
var slackMrId map[string]string

type MergeHook struct {
	Object_kind string
	User        struct {
		Name     string
		Username string
	}
	Project struct {
		Id         int
		Name       string
		Avatar_url string
	}
	Repository struct {
		Name string
	}
	Object_attributes struct {
		Id           int
		Iid          int
		Title        string
		State        string
		Merge_status string
		Description  string
		Url          string
		Action       string
	}
}

func parse(w http.ResponseWriter, r *http.Request) {
	log.Println("Request")
	if r.Header.Get("X-Gitlab-Event") != "" {
		value, _ := header.ParseValueAndParams(r.Header, "X-Gitlab-Event")
		log.Println("X-Gitlab-Event: " + value)
		if strings.Contains(value, "merge") {
			parseHook(w, r)
		} else {
			msg := "Content-Type header is not merge"
			http.Error(w, msg, http.StatusUnsupportedMediaType)
			return
		}
	}
}

func isApproved(project string, idd string) bool {
	url := "https://" + os.Getenv("GITLAB-DOMAIN") + "/api/v4/projects/" + project + "/merge_requests/" + idd + "/approval_state"
	spaceClient := http.Client{
		Timeout: time.Second * 5, // Timeout after 2 seconds
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		print(err.Error())
		return false
	}

	req.Header.Set("PRIVATE-TOKEN", os.Getenv("PRIVATE-TOKEN"))

	res, getErr := spaceClient.Do(req)
	if getErr != nil {
		print(getErr.Error())
		return false
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		print(readErr.Error())
		return false
	}
	if strings.Contains(string(body), `{"message":`) {
		print(string(body) + "\n")
	}
	if strings.Contains(string(body), `"approved":true`) {
		return true
	}
	return false
}
func parseHook(w http.ResponseWriter, r *http.Request) {

	// 1MB is max body size
	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	//	requestDump, _ := httputil.DumpRequest(r, true)
	//	log.Println(string(requestDump))

	dec := json.NewDecoder(r.Body)

	var mr MergeHook
	err := dec.Decode(&mr)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError

		switch {
		// interpolates the location of the problem to make debug easier
		case errors.As(err, &syntaxError):
			msg := fmt.Sprintf("Request body contains badly-formed JSON (at position %d)", syntaxError.Offset)
			http.Error(w, msg, http.StatusBadRequest)

		// https://github.com/golang/go/issues/25956.
		case errors.Is(err, io.ErrUnexpectedEOF):
			msg := fmt.Sprintf("Request body contains badly-formed JSON")
			http.Error(w, msg, http.StatusBadRequest)

		case errors.As(err, &unmarshalTypeError):
			msg := fmt.Sprintf("Request body contains an invalid value for the %q field (at position %d)", unmarshalTypeError.Field, unmarshalTypeError.Offset)
			http.Error(w, msg, http.StatusBadRequest)

		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			msg := fmt.Sprintf("Request body contains unknown field %s", fieldName)
			http.Error(w, msg, http.StatusBadRequest)

		case errors.Is(err, io.EOF):
			msg := "Request body must not be empty"
			http.Error(w, msg, http.StatusBadRequest)

		case err.Error() == "http: request body too large":
			msg := "Request body must not be larger than 1MB"
			http.Error(w, msg, http.StatusRequestEntityTooLarge)

		// Otherwise default to logging the error and sending a 500 Internal
		default:
			log.Println(err.Error())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		msg := "Request body must only contain a single JSON object"
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	//fmt.Fprintf(w, "MR: %+v", mr)
	log.Println(fmt.Sprintf("MR: %+v", mr))
	message := ""
	smessage := ""
	if mr.Object_attributes.State == "opened" {
		approve := isApproved(strconv.Itoa(mr.Project.Id), strconv.Itoa(mr.Object_attributes.Iid))
		if approve {
			message = fmt.Sprintf("<b>[✅APPROVED]</b> %s<b> %s</b> \n %s ", mr.User.Name, mr.Object_attributes.Title, mr.Object_attributes.Url)
			smessage = fmt.Sprintf("[✅ *APPROVED*] %s %s \n %s ", mr.User.Name, mr.Object_attributes.Title, mr.Object_attributes.Url)
		} else {
			message = fmt.Sprintf("<b>[🟡%s]</b> %s<b> %s</b> \n %s ", strings.ToUpper(mr.Object_attributes.State), mr.User.Name, mr.Object_attributes.Title, mr.Object_attributes.Url)
			smessage = fmt.Sprintf("[🟡 *%s*] %s %s @here \n %s ", strings.ToUpper(mr.Object_attributes.State), mr.User.Name, mr.Object_attributes.Title, mr.Object_attributes.Url)
		}
		if val, ok := MrId[strconv.Itoa(mr.Object_attributes.Id)+":"+strconv.Itoa(mr.Object_attributes.Iid)]; ok {
			go editMessage(val, message)
		} else {
			go func(msg string) {
				tid := sendMessage(msg)
				MrId[strconv.Itoa(mr.Object_attributes.Id)+":"+strconv.Itoa(mr.Object_attributes.Iid)] = tid
			}(message)
		}
		if val, ok := slackMrId[strconv.Itoa(mr.Object_attributes.Id)+":"+strconv.Itoa(mr.Object_attributes.Iid)]; ok {
			go editSlackMessage(val, smessage)
		} else {
			go func(msg string) {
				slackid := sendSlackMessage(msg)
				slackMrId[strconv.Itoa(mr.Object_attributes.Id)+":"+strconv.Itoa(mr.Object_attributes.Iid)] = slackid
			}(smessage)
		}
	} else if mr.Object_attributes.State == "merged" {
		message = fmt.Sprintf("[☑️ MERGED] <strike> %s %s </strike>\n %s ", mr.User.Name, mr.Object_attributes.Title, mr.Object_attributes.Url)
		smessage = fmt.Sprintf("[☑️ MERGED] ~%s %s~ \n %s ", mr.User.Name, mr.Object_attributes.Title, mr.Object_attributes.Url)
		if val, ok := MrId[strconv.Itoa(mr.Object_attributes.Id)+":"+strconv.Itoa(mr.Object_attributes.Iid)]; ok {
			go editMessage(val, message)
		} else {
			go func(msg string) {
				tid := sendMessage(msg)
				MrId[strconv.Itoa(mr.Object_attributes.Id)+":"+strconv.Itoa(mr.Object_attributes.Iid)] = tid
			}(message)
		}
		if val, ok := slackMrId[strconv.Itoa(mr.Object_attributes.Id)+":"+strconv.Itoa(mr.Object_attributes.Iid)]; ok {
			go editSlackMessage(val, smessage)
		} else {
			go func(msg string) {
				slackid := sendSlackMessage(msg)
				slackMrId[strconv.Itoa(mr.Object_attributes.Id)+":"+strconv.Itoa(mr.Object_attributes.Iid)] = slackid
			}(smessage)
		}
	} else if mr.Object_attributes.State == "closed" {
		message = fmt.Sprintf("[☑️ CLOSED] <strike> %s %s </strike>\n %s ", mr.User.Name, mr.Object_attributes.Title, mr.Object_attributes.Url)
		smessage = fmt.Sprintf("[☑️ CLOSED] ~%s %s~ \n %s ", mr.User.Name, mr.Object_attributes.Title, mr.Object_attributes.Url)
		if val, ok := MrId[strconv.Itoa(mr.Object_attributes.Id)+":"+strconv.Itoa(mr.Object_attributes.Iid)]; ok {
			go editMessage(val, message)
		} else {
			go func(msg string) {
				tid := sendMessage(msg)
				MrId[strconv.Itoa(mr.Object_attributes.Id)+":"+strconv.Itoa(mr.Object_attributes.Iid)] = tid
			}(message)
		}
		if val, ok := slackMrId[strconv.Itoa(mr.Object_attributes.Id)+":"+strconv.Itoa(mr.Object_attributes.Iid)]; ok {
			go editSlackMessage(val, smessage)
		} else {
			go func(msg string) {
				slackid := sendSlackMessage(msg)
				slackMrId[strconv.Itoa(mr.Object_attributes.Id)+":"+strconv.Itoa(mr.Object_attributes.Iid)] = slackid
			}(smessage)
		}
	}
}
func checkEnv() {
	isRequired := []string{
		"PRIVATE-TOKEN",     //gitlab-token
		"TELEGRAMM-TOKEN",   //23234234234:ayanxcsjdrghgs-jilksa
		"TELEGRAMM-CHANNEL", //int
		"SLACK-TOKEN",       // doss-3452342-234234234234234234-23234234234
		"SLACK-CHANNEL",     // CJ743SD6
		"GITLAB-DOMAIN",     // gitlab.mycompany.com
	}
	for e := range isRequired {
		if len(os.Getenv(isRequired[e])) < 1 {
			fmt.Printf("There is no ENV %s \n should be declared %v \n", isRequired[e], isRequired)
			os.Exit(0)
		}
	}

}

func main() {
	checkEnv()
	MrId = make(map[string]int)
	slackMrId = make(map[string]string)
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", parse)

	err := http.ListenAndServe("0.0.0.0:4000", mux)
	log.Fatal(err)
}
