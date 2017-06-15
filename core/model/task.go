package model

import (
	log "github.com/cihub/seelog"
	"github.com/medcl/gopa/core/errors"
	"github.com/medcl/gopa/core/store"

	"bytes"
	"fmt"
	"github.com/medcl/gopa/core/pipeline"
	"github.com/medcl/gopa/core/util"
	"strconv"
	"strings"
	"time"
)

type TaskStatus int

const TaskCreated TaskStatus = 0
const TaskFetchFailed TaskStatus = 2
const TaskFetchSuccess TaskStatus = 3

type Seed struct {
	Url       string `storm:"index" json:"url,omitempty" gorm:"type:not null;varchar(500)"` // the seed url may not cleaned, may miss the domain part, need reference to provide the complete url information
	Reference string `json:"reference_url,omitempty"`
	Depth     int    `storm:"index" json:"depth,omitempty"`
	Breadth   int    `storm:"index" json:"breadth,omitempty"`
}

func (this Seed) Get(url string) Seed {
	task := Seed{}
	task.Url = url
	task.Reference = ""
	task.Depth = 0
	task.Breadth = 0
	return task
}

func (this Seed) MustGetBytes() []byte {

	bytes, err := this.GetBytes()
	if err != nil {
		panic(err)
	}
	return bytes
}

var delimiter = "|#|"

func (this Seed) GetBytes() ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprint(this.Breadth))
	buf.WriteString(delimiter)
	buf.WriteString(fmt.Sprint(this.Depth))
	buf.WriteString(delimiter)
	buf.WriteString(this.Reference)
	buf.WriteString(delimiter)
	buf.WriteString(this.Url)

	return buf.Bytes(), nil
}

func TaskSeedFromBytes(b []byte) Seed {
	task, err := fromBytes(b)
	if err != nil {
		panic(err)
	}
	return task
}

func fromBytes(b []byte) (Seed, error) {

	str := string(b)
	array := strings.Split(str, delimiter)
	task := Seed{}
	i, _ := strconv.Atoi(array[0])
	task.Breadth = i
	i, _ = strconv.Atoi(array[1])
	task.Depth = i
	task.Reference = array[2]
	task.Url = array[3]

	return task, nil
}

func NewTaskSeed(url, ref string, depth int, breadth int) Seed {
	task := Seed{}
	task.Url = url
	task.Reference = ref
	task.Depth = depth
	task.Breadth = breadth
	return task
}

type Task struct {
	Seed
	ID            string          `storm:"id,unique" json:"id" gorm:"not null;unique;primary_key"`
	Host          string          `storm:"index" json:"-"`
	Schema        string          `json:"schema,omitempty"`
	OriginalUrl   string          `json:"original_url,omitempty"`
	Phrase        pipeline.Phrase `storm:"phrase" json:"phrase"`
	Status        TaskStatus      `storm:"index" json:"status"`
	Message       string          `storm:"inline" json:"-"`
	CreateTime    *time.Time      `storm:"index" json:"created,omitempty"`
	UpdateTime    *time.Time      `storm:"index" json:"updated,omitempty"`
	LastFetchTime *time.Time      `storm:"index" json:"-"`
	LastCheckTime *time.Time      `storm:"index" json:"-"`
	NextCheckTime *time.Time      `storm:"index" json:"-"`

	SnapshotVersion int    `json:"-"`
	SnapshotID      string `json:"-"` //Last Snapshot's ID
	SnapshotHash    string `json:"-"` //Last Snapshot's Hash
	SnapshotSimHash string `json:"-"` //Last Snapshot's Simhash
}

func CreateTask(task *Task) error {
	log.Trace("start create crawler task, ", task.Url)
	time := time.Now()
	task.ID = util.GetIncrementID("task")
	task.Status = TaskCreated
	task.CreateTime = &time
	task.UpdateTime = &time
	err := store.Save(task)
	if err != nil {
		log.Debug(task.ID, ", ", err)
	}
	return err
}

func UpdateTask(task *Task) {
	log.Trace("start update crawler task, ", task.Url)
	time := time.Now()
	task.UpdateTime = &time
	err := store.Update(task)
	if err != nil {
		panic(err)
	}
}

func DeleteTask(id string) error {
	log.Trace("start delete crawler task: ", id)
	task := Task{ID: id}
	err := store.Delete(&task)
	if err != nil {
		log.Debug(id, ", ", err)
	}
	return err
}

func GetTask(id string) (Task, error) {
	log.Trace("start get seed: ", id)
	task := Task{}
	err := store.GetBy("id", id, &task)
	if err != nil {
		log.Debug(id, ", ", err)
	}
	if len(task.ID) == 0 || task.CreateTime == nil {
		panic(errors.New("not found," + id))
	}

	return task, err
}

func GetTaskByField(k, v string) (Task, error) {
	log.Trace("start get seed: ", k, ", ", v)
	task := Task{}
	err := store.GetBy(k, v, &task)
	if err != nil {
		log.Debug(k, ", ", err)
	}
	return task, err
}

func GetTaskList(from, size int, domain string) (int, []Task, error) {
	log.Tracef("start get crawler tasks, %v-%v, %v", from, size, domain)
	var tasks []Task
	queryO := store.Query{Sort: "create_time desc", From: from, Size: size}
	if len(domain) > 0 {
		queryO.Filter = &store.Cond{Name: "host", Value: domain}
	}
	err, result := store.Search(&tasks, &queryO)
	if err != nil {
		log.Trace(err)
	}
	return result.Total, tasks, err
}

func GetPendingFetchTasks() (int, []Task, error) {
	log.Trace("start get all crawler tasks")
	var tasks []Task
	queryO := store.Query{Sort: "create_time desc", Filter: &store.Cond{Name: "phrase", Value: 1}}
	err, result := store.Search(&tasks, &queryO)
	if err != nil {
		log.Trace(err)
	}
	return result.Total, tasks, err
}
