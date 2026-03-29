// Package api provides the HTTP API for the FakeNumber DNO system.
//
//	@title						FakeNumber DNO API
//	@version					1.0
//	@description				Do Not Originate database API for preventing illegal robocalls and caller ID spoofing.
//	@description				Carriers query the DNO database in real-time to block spoofed calls.
//
//	@contact.name				FakeNumber DNO
//	@contact.url				https://github.com/drewkosta/realNumberDNOClone
//
//	@host						localhost:8080
//	@BasePath					/api/v1
//
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				JWT Bearer token (prefix with "Bearer ")
//
//	@securityDefinitions.apikey	APIKeyAuth
//	@in							header
//	@name						X-API-Key
//	@description				Organization API key for machine-to-machine access
package api
