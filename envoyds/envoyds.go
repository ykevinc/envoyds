package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"github.com/ykevinc/envoyds"
)

func main() {
	configPath := "envoyds.conf"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}
	c := envoyds.ReadConfig(configPath)
	r, err := envoyds.NewRouter(c.Environment, c.RedisHost, c.RedisPort)
	if err != nil {
		log.Fatal(err)
	}
	server := http.Server{
		Addr:    fmt.Sprintf(":%d", c.Port),
		Handler: r,
	}
	fmt.Printf("Ready to Listen on %d\n", c.Port)
	if err = server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}