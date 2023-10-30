package main

import (
	"fmt"

	"github.com/spf13/viper"
)

func main() {
	loadConfig()
	tri := initTri()
	// TODO DEBUG
	tri.printAll()
	messenger := initMessenger()
	messenger.setTri(tri)
	go messenger.process()
	connectToBybit(messenger)
}

func loadConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Fatal error config file: %w \n", err))
	}
	fmt.Println("Config has been loaded successfully!")
	fmt.Printf("ENV: %s\n", viper.Get("ENV"))
}
