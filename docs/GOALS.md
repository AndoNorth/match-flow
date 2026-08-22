# Goals

## Purpose

MatchFlow exists to give experienced backend engineers hands-on practice
with the patterns used in modern distributed systems, using a domain
(live sports events) that is simple enough to stay out of the way.

## What Engineers Should Practice Here

- Designing and building Go services with clear boundaries
- Distributing events through Redis rather than a heavyweight broker
- Choosing between REST and gRPC for different communication needs
- Handling concurrent event processing correctly
- Pushing real-time updates to connected clients
- Instrumenting services with distributed tracing, metrics, and logging
- Running services in containers and orchestrating them locally and in Kubernetes
- Writing automated tests that give confidence across service boundaries

## Non-Goals

- MatchFlow is not a production sports data product. Match and event data
  is simulated, not sourced from a real provider.
- MatchFlow is not an exhaustive microservices reference architecture.
  It intentionally uses a small number of services.
- MatchFlow does not aim to showcase every distributed systems technology.
  Kafka, RabbitMQ, NATS, a service mesh, Terraform, and ArgoCD are
  explicitly out of scope for the core build (see
  [ARCHITECTURE.md](ARCHITECTURE.md#future-enhancements) for where they
  might fit later).
- MatchFlow does not prescribe a single "correct" implementation for any
  service. Responsibilities are defined; internal implementation is left
  to the engineer building it.
- MatchFlow has no user accounts, authentication, or authorization. The
  practice surface is the backend services and the realtime distribution
  path to the client, not a full product with users. The frontend is a
  read-only client of live match data, not a multi-tenant application.

## Audience

Backend engineers who are comfortable with a general-purpose language and
want structured practice with Go, Redis, real-time systems, and
cloud-native operations, in that rough order.
