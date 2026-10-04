module github.com/nexus/graph-engine

go 1.25.0

require nexus/shared v0.0.0

require github.com/neo4j/neo4j-go-driver/v5 v5.28.5

require (
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/rabbitmq/amqp091-go v1.15.0 // indirect
)

replace nexus/shared => ../shared
