# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed
- Extract connection pool into dedicated package

## [0.2.0] - 2026-02-18

### Added
- `streamline_topic` resource for topic management
- `streamline_schema` resource for schema registry management
- `streamline_acl` resource for access control management
- `streamline_cluster` data source for cluster information
- `streamline_topics` data source for listing topics
- Acceptance tests for all resources and data sources
- Terraform Plugin Framework v1.5 based provider
