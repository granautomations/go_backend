# Automation Backend

A Go backend for a web application and connected devices.

## Planned capabilities

- HTTP API for the web application
- PostgreSQL persistence with CRUD operations
- MQTT integration for receiving and processing device messages
- Device and sensor-reading management

## Current endpoints

- `GET /health`
- `GET /devices/{id}`
- `GET /devices/{id}/readings/latest`

## Run tests

```bash
go test ./...
```