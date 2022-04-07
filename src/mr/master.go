package mr

import (
	"log"
	"sync"
	"time"
)
import "net"
import "os"
import "net/rpc"
import "net/http"

type Master struct {
	// Your definitions here.
	mapTasks    []*Task
	reduceTasks []*Task
	nReduce     int
	mapDone     bool
	reduceDone  bool
	mapFiles    []string
}

//心跳时间，但并不没有采取心跳连接，而是单个task能有的最多时间
const tolerance time.Duration = 10 * time.Second

//互斥锁
var mu sync.Mutex

// Your code here -- RPC handlers for the worker to call.
func (m *Master) initTasks() {
	for i, filename := range m.mapFiles {
		m.mapTasks = append(m.mapTasks, NewTask(MapPhase, Todo, i, -1, m.nReduce, filename))
	}
	for i := 0; i < m.nReduce; i++ {
		m.reduceTasks = append(m.reduceTasks, NewTask(ReducePhase, Todo, len(m.mapFiles), i, m.nReduce, "r"))
	}
}

func (m *Master) RequestTask(args *TaskRequestArgs, reply *TaskRequestReply) error {
	//请求task的互斥锁，粒度大概可以更小一点
	mu.Lock()
	defer mu.Unlock()
	var find bool
	//all finished发
	if m.reduceDone {
		reply.ok = false
		reply.reason = finished
		return nil
	}
	if !m.mapDone {
		for _, mapTask := range m.mapTasks {
			if mapTask.State == Doing && time.Now().Sub(mapTask.StartTime) > tolerance {
				mapTask.State = Todo
			}
			if mapTask.State == Todo {
				mapTask.StartTime = time.Now()
				reply.ok = true
				reply.task = mapTask
				find = true
			}
		}
	} else {
		for _, reduceTask := range m.reduceTasks {
			if reduceTask.State == Doing && time.Now().Sub(reduceTask.StartTime) > tolerance {
				reduceTask.State = Todo
			}
			if reduceTask.State == Todo {
				reduceTask.StartTime = time.Now()
				reply.ok = true
				reply.task = reduceTask
				find = true
			}
		}
	}
	if !find {
		reply.ok = false
		reply.reason = wait
	}
	return nil
}

//
// start a thread that listens for RPCs from worker.go
//
func (m *Master) server() {
	rpc.Register(m)
	rpc.HandleHTTP()
	sockname := masterSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

//
// main/mrmaster.go calls Done() periodically to find out
// if the entire job has finished.
//
func (m *Master) Done() bool {
	ret := false

	// Your code here.

	return ret
}

//
// create a Master.
// main/mrmaster.go calls this function.
// nReduce is the number of reduce tasks to use.
//
func MakeMaster(files []string, nReduce int) *Master {
	m := Master{}
	m.mapFiles = files
	m.nReduce = nReduce
	m.initTasks()
	// Your code here.

	m.server()
	return &m
}
