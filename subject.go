package main

type Subject struct {
	Name string
}

type SubjectListener struct {
	Subject Subject
	Client  ConnectedClient
}
