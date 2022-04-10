package mr

import (
	"log"
	"time"
)

const debug = true

//步骤
type TaskPhase int

const MapPhase TaskPhase = 0
const ReducePhase TaskPhase = 1

//状态
type TaskState int

const Todo TaskState = 0
const Doing TaskState = 1
const Done TaskState = 2
const Failed TaskState = 3

//返回理由
type Reason int

const wait Reason = 0
const finished Reason  = 1

//有没有必要保存task对应的workerid？
type Task struct {
	Phase        TaskPhase
	State        TaskState
	//对于map任务是map编号
	// 对于reduce来说，因为reduce需要获取所有map任务产出到对应reduce区间上的内容
	// 所以是map全部编号
	MapNumber    int
	//reduce需要
	ReduceNumber int
	Nreduce      int
	StartTime	time.Time
	//map需要
	FileName	string
}

//constructor
func NewTask(phase TaskPhase, state TaskState, mapNumber int, reduceNumber int, nreduce int, fileName string) *Task {
	return &Task{Phase: phase, State: state, MapNumber: mapNumber, ReduceNumber: reduceNumber, Nreduce: nreduce,  FileName: fileName}
}

//log
func DPrint(str string, v ...interface{})  {
	if debug {
		log.Printf(str,v)
	}
}