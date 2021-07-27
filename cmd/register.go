/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"context"
	"fmt"
	"log"

	"github.com/BENSARI-Fathi/imagehub/v1/pb"
	"github.com/howeyc/gopass"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

// registerCmd represents the register command
var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "register a new user",
	Long: `Create a user account by providing the 
username, email and password`,
	Args:                  cobra.ExactArgs(0),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		register()
	},
}

func init() {
	rootCmd.AddCommand(registerCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// registerCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// registerCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func register() {

	var username, email string

	cc, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Error while trying to connect %v", err)
	}
	defer cc.Close()
	c := pb.NewImageReposClient(cc)

	// ask the client to provide credentials
	fmt.Print("Username: ")
	fmt.Scanln(&username)
	fmt.Print("Email: ")
	fmt.Scanln(&email)
	fmt.Print("Password: ")
	password, _ := gopass.GetPasswd()
	fmt.Print("Confirm password: ")
	password2, _ := gopass.GetPasswd()
	request := &pb.RegisterRequest{
		Username:  username,
		Email:     email,
		Password:  string(password),
		Password2: string(password2),
	}
	resp, err := c.Register(context.Background(), request)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("User successfully created %v\n", resp)
}
