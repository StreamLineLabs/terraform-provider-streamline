# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).



## [Unreleased]

### Added
- `streamline_consumer_group` resource — manage consumer groups via Terraform (CRUD + import)
- Client: `ListConsumerGroups()`, `DescribeConsumerGroup()`, `DeleteConsumerGroup()` methods
- Consumer group resource supports `group_id`, computed `state` and `members` attributes

- refactor: extract common CRUD helpers (2026-03-06)
- fix: resolve state drift detection for ACL resources (2026-03-06)
- feat: add topic retention policy resource (2026-03-06)
- **Testing**: add plan-only tests for resource changes
- **Fixed**: correct import state for existing topics
- **Documentation**: regenerate provider documentation from schema
- **Added**: implement data source for cluster info
- **Fixed**: handle API timeout in resource read operations
- **Changed**: update terraform-plugin-framework dependency
- **Changed**: extract common CRUD patterns into helpers
- **Testing**: add acceptance tests for provider configuration
- **Fixed**: resolve state drift detection for ACL resources
- **Added**: add streamline_topic resource implementation

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
- fix: handle null values in resource plan comparison
