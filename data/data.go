package data

import (
	"sync"
)

type Link struct {
	Name  string
	Links []Link
}

type ExecutionContext struct {
	Wg *sync.WaitGroup
}

type Broker struct {
	Status  chan string
	Workers chan Worker
}

type Worker struct {
	Work  chan string
	Links []string
	Name  string
	Ready chan bool
}
