package main

type Subject struct {
	name string
}

type SubjectListener struct {
	subject Subject
	client  ConnectedClient
}
