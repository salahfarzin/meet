# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

A Go gRPC + REST (grpc-gateway) microservice for managing **meets** (appointments). MySQL-backed, dual-server (gRPC + HTTP gateway), auth delegated to an external auth service. Detailed guidance is split into topic files:

- @.claude/context/commands.md — build, test, lint, proto regen, migrations
- @.claude/context/architecture.md — dual-server boot, request flow, layering, delegated auth
- @.claude/context/domain.md — Meet entity, RPCs/REST routes, business rules
- @.claude/context/dashboard-integration.md — how the dashboard-v1 events section consumes this service
