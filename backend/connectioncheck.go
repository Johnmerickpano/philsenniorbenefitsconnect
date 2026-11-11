package main

import (
    "fmt"
    "github.com/gocql/gocql"
)

func main() {
    cluster := gocql.NewCluster("host.docker.internal")
    cluster.Port = 9042
    cluster.Keyspace = "psbc"
    cluster.Timeout = 10 * 1000000000
    cluster.ConnectTimeout = 10 * 1000000000

    session, err := cluster.CreateSession()
    if err != nil {
        fmt.Printf("CONNECTION FAILED: %v\n", err)
        return
    }
    defer session.Close()

    fmt.Println("GO DRIVER CONNECTED SUCCESSFULLY")
}
