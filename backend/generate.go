// Package deezmails contains repository-level generation directives.
package deezmails

// @title DeezMails API
// @version 0.1.0
// @description Manage OAuth-connected Gmail and Microsoft mailboxes, proxies, and synced messages.
// @host localhost:8080
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
//
//go:generate go tool swag init -g generate.go --parseInternal -o docs
