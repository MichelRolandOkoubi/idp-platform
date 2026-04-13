# Architecture Overview

The IDP is designed as a modular platform to enable self-service infrastructure provisioning and management.

## Components

### 1. idpctl (CLI)
Written in Rust, it provides a fast and reliable interface for developers to interact with the platform.

### 2. Control Plane
The heart of the platform, written in Go. it exposes an API for resource management and orchestrates provisioning.

### 3. ML Cost Engine
A Python service that analyzes cloud spend, predicts future costs, and detects anomalies using ML models.

### 4. Infrastructure & GitOps
Infrastructure is managed via Terraform and Ansible, while applications are deployed using ArgoCD.
