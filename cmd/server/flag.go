package main

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
)

type flagAddr struct {
	Host string
	Port int
}

type ConfigAddr struct {
	Addr *flagAddr
}

func (f *flagAddr) Set(s string) error {
	hp := strings.Split(s, ":")

	if len(hp) < 2 {
		return fmt.Errorf("Need to be in format host:port")
	}
	port, err := strconv.Atoi(hp[1])

	if err != nil {
		return fmt.Errorf("Error")
	}
	f.Port = int(port)
	f.Host = hp[0]

	return nil
}

func (f *flagAddr) String() (s string) {
	return f.Host + ":" + strconv.Itoa(f.Port)
}

func ParseFlag() *ConfigAddr {
	cfg := &ConfigAddr{
		Addr: &flagAddr{
			Host: "localhost",
			Port: 8080,
		}}
	flag.Var(cfg.Addr, "a", "Net address host:port")

	flag.Parse()
	return cfg
}
