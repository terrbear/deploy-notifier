package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/slack-go/slack"
	"terrbear.io/deploy-notifier/internal/env"
)

type deployment struct {
	lock      sync.Mutex
	projects  []*project
	link      string
	startTime time.Time
	timestamp string // slack timestamp
	done      bool
}

type project struct {
	name      string
	status    string
	startTime time.Time
	endTime   time.Time
	id        int
	failed    bool
}

const noop = "not deploying"
const deploying = "deploying"
const building = "building"
const succeeded = "succeeded"

func (d *deployment) project(pName string) *project {
	d.lock.Lock()
	defer d.lock.Unlock()

	for _, project := range d.projects {
		if project.name == pName {
			return project
		}
	}

	p := &project{
		name:      pName,
		id:        <-newID,
		startTime: time.Now(),
	}

	d.projects = append(d.projects, p)
	return p
}

func (d *deployment) notDeploying(pName string) {
	p := d.project(pName)
	p.status = noop
}

func (d *deployment) startDeploying(pName string) {
	p := d.project(pName)
	p.status = deploying
}

func (d *deployment) startBuilding(pName string) {
	p := d.project(pName)
	p.status = building
}

func (d *deployment) failedDeploying(pName string) {
	p := d.project(pName)
	p.failed = true
	p.endTime = time.Now()
}

func (d *deployment) succeededDeployment(pName string) {
	p := d.project(pName)
	p.status = succeeded
	p.endTime = time.Now()
}

var newID = make(chan int)

func idGenerator() {
	id := 0
	for {
		newID <- id
		id++
	}
}

var nameIDMap = make(map[string]int)

const failedColor = "#ff4500"

var colorMap = map[string]string{
	noop:      "#aaa",
	deploying: "#ffa500",
	building:  "#ffa500",
	succeeded: "#0b0",
}

func formatDuration(d time.Duration) string {
	seconds := int(d.Seconds())
	return fmt.Sprintf("%02d:%02d", seconds/60, seconds%60)
}

func (p *project) statusString() string {
	name := strings.Title(p.name)
	status := ""
	switch p.status {
	case noop:
		status = " is not deploying (no changes)"
	case building:
		duration := time.Now().Sub(p.startTime)
		status = fmt.Sprintf(" building (%s)", formatDuration(duration))
	case deploying:
		duration := time.Now().Sub(p.startTime)
		status = fmt.Sprintf(" deploying (%s)", formatDuration(duration))
	case succeeded:
		duration := p.endTime.Sub(p.startTime)
		status = fmt.Sprintf(" succeeded (took %s)", formatDuration(duration))
	}

	if p.failed {
		return name + " FAILED " + status
	}

	return name + status
}

func (p *project) success() bool {
	return !p.failed && (p.status == succeeded || p.status == noop)
}

func (d *deployment) broadcast() {
	d.lock.Lock()
	defer d.lock.Unlock()

	if len(d.projects) == 0 {
		return
	}

	blocks := make([]slack.Attachment, len(d.projects))

	success := true
	for i, project := range d.projects {
		color := colorMap[project.status]
		if project.failed {
			color = failedColor
		}
		blocks[i] = slack.Attachment{Color: color, Text: project.statusString(), ID: project.id}
		success = project.success() && success
	}

	if d.done {
		duration := time.Now().Sub(d.startTime)
		color := failedColor
		status := fmt.Sprintf("Failed (took %s)", formatDuration(duration))
		if success {
			color = colorMap[succeeded]
			status = fmt.Sprintf("Succeeded (took %s)", formatDuration(duration))
		}

		blocks = append(blocks, slack.Attachment{Color: color, Text: status, ID: 42})
	}

	api := slack.New(env.SlackToken())
	channelID := env.ChannelID()

	headerMsg := os.Getenv("SLACK_HEADER")
	header := slack.MsgOptionText(headerMsg, false)

	if d.timestamp == "" {
		_, s2, _ := api.PostMessage(channelID, header, slack.MsgOptionAttachments(blocks...))
		d.timestamp = s2
	} else {
		api.UpdateMessage(channelID, d.timestamp, header, slack.MsgOptionAttachments(blocks...))
	}
}

func (d *deployment) periodicBroadcast() {
	t := time.NewTicker(15 * time.Second)
	for {
		<-t.C
		if d.done {
			return
		}
		d.broadcast()
	}
}

func main() {
	go idGenerator()
	d := deployment{
		startTime: time.Now(),
		projects:  make([]*project, 0),
	}
	r := gin.Default()
	go d.periodicBroadcast()

	r.GET("/building/:project", func(c *gin.Context) {
		d.startBuilding(c.Param("project"))
		d.broadcast()
		c.String(http.StatusOK, "ok")
	})

	r.GET("/deploying/:project", func(c *gin.Context) {
		d.startDeploying(c.Param("project"))
		d.broadcast()
		c.String(http.StatusOK, "ok")
	})

	r.GET("/failed/:project", func(c *gin.Context) {
		d.failedDeploying(c.Param("project"))
		d.broadcast()
		c.String(http.StatusOK, "ok")
	})

	r.GET("/succeeded/:project", func(c *gin.Context) {
		d.succeededDeployment(c.Param("project"))
		d.broadcast()
		c.String(http.StatusOK, "ok")
	})

	r.GET("/noop/:project", func(c *gin.Context) {
		d.notDeploying(c.Param("project"))
		d.broadcast()
		c.String(http.StatusOK, "ok")
	})

	r.GET("/done", func(c *gin.Context) {
		d.done = true
		d.broadcast()
		c.String(http.StatusOK, "ok")
	})

	r.Run(":8085")
}
