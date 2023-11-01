package main

import (
	"fmt"

	"github.com/spf13/viper"
)

func main() {
	loadEnvConfig()
	tri := initTri()
	// TODO DEBUG
	tri.printAllCombinations()
	runner := initRunner()
	runner.setTri(tri)
	go runner.feed()

	// Have to be after initTri as it will set klines
	connectToBybit(runner)
}

func loadEnvConfig() {
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
