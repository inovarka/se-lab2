package main

type PutRequest struct {
	Value []byte
}

type GetResponse struct {
	Key   string
	Value []byte
}
