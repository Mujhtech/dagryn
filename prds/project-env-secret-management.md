# Project Env & Secret Management

## TL;DR
- Build a first-class project environment and secret management system across CLI, API, and dashboard settings.
- Users can seed secrets from CLI, pull/inject them for local runs, and manage project env values in settings with branch-aware overrides.
- Support multiple secret backends (encrypted DB, AWS Secrets Manager, Google Secret Manager, Cloudflare) with unified policy, auditing, and runtime injection.

## Background
- Today, env values are primarily task-level config in workflow files, which is not ideal for secure shared secret lifecycle management.
- Project settings currently cover general metadata, Git/repository, and API keys, but not centralized env/secret administration.
- Existing product patterns already include one-time secret exposure and encrypted token storage, giving a foundation for secret UX and security controls.

## Problem & Target Users
- Platform engineers and backend teams need a secure, reusable, project-scoped secret source instead of committing or manually syncing env values.
- Developers need a reliable way to pull and inject project env for local execution without ad-hoc scripts.
- Security/compliance stakeholders need provider flexibility, access controls, and auditability across secret operations.
- Team leads need branch-specific override support for preview/feature workflows without duplicating full environment configs.

## Goals & Success Metrics
- Enable full env lifecycle: set/seed, list, pull, inject, rotate, delete for project-scoped variables.
- Support all required providers in GA: DB encrypted storage, AWS, GCP, and Cloudflare secret store.
- Provide branch override resolution with deterministic precedence: `branch > environment > project default`.
- Achieve high adoption and safety targets: majority of active projects store at least one secret in managed env, and zero plaintext secret exposure via settings UI/API list responses.

## Solution Overview
- Introduce a unified Project Env Registry where each key has metadata (scope, environment, branch, required flag, provider).
- Split metadata from secret material using separate records: env mapping table plus secret storage table with provider references/versioning.
- Add a provider abstraction so CLI/runtime/API use a single interface for put/get/rotate/delete across DB/AWS/GCP/Cloudflare backends.
- Resolve env values at run time and local-run injection through precedence rules, then mask secret values in logs and telemetry.

## User Experience
- **CLI set/seed**: users add one key or bulk import from dotenv-like inputs into project settings, choosing provider and secret/non-secret classification.
- **CLI pull/inject**: users fetch project env for a target environment/branch and inject into local execution; default behavior overwrites matching local values.
- **Project settings**: users manage env records in a dedicated tab with filtering by environment/branch, provider badge, required flag, and last-updated metadata.
- **Secret visibility**: secret values are never revealed in UI after creation; authorized CLI commands can fetch plaintext for local use.
- **Runtime behavior**: missing required env triggers warning and run continues (per requested policy), while surfaced clearly in run diagnostics.

## Requirements
- Add project settings area for env/secrets with CRUD, filtering by environment and branch, and metadata-only list responses for secrets.
- Add CLI command family for env management (set, seed, list, pull, inject, delete, rotate) with project/environment/branch targeting.
- Implement provider adapters for DB, AWS, GCP, and Cloudflare with unified error model and health validation.
- Persist env metadata and secret storage references in separate tables; support secret versioning and provider-specific references.
- Enforce precedence rule `branch > environment > default` for resolution during local and remote runs.
- Support required/non-required flags; missing required keys emit structured warnings and do not hard-fail runs.
- Ensure UI cannot reveal secret plaintext post-creation; CLI reveal is permission-gated and fully audited.
- Add audit logging for create/update/delete/rotate/reveal/pull operations with actor, project, key, provider, and timestamp metadata.

## Out of Scope
- Auto-migration of every existing task-level env value into project env without user confirmation.
- Secret scanning/remediation of historical Git history.
- Building a custom external KMS product beyond provider integrations and DB envelope encryption support.

## Open Questions
- Should branch scope support wildcard patterns (for example `feature/*`) in GA or only exact branch names initially?
- What are hard product limits at launch (max variables per project, max value size, provider rate-limit handling strategy)?
- Should pull/inject support partial key selection profiles (for example `--profile ci` or tagged key groups) in first release?
- Which RBAC roles can perform CLI reveal and rotate by default in team workspaces?

## API Contract Draft
- `POST /api/v1/projects/{projectId}/env-vars`: create or upsert a project env record with scope (`environment`, `branch`), provider, and required flag.
- `GET /api/v1/projects/{projectId}/env-vars`: list env metadata with filters (`environment`, `branch`, `key`, `provider`, `is_secret`); secret values are never returned.
- `PATCH /api/v1/projects/{projectId}/env-vars/{envVarId}`: update metadata (scope, required, provider reference, description, enabled state).
- `DELETE /api/v1/projects/{projectId}/env-vars/{envVarId}`: remove env record and optionally detach underlying provider reference.
- `POST /api/v1/projects/{projectId}/env-vars/{envVarId}/rotate`: rotate secret value and persist new provider version/reference.
- `POST /api/v1/projects/{projectId}/env-vars/resolve`: resolve effective env for runtime/local use using precedence `branch > environment > default`.
- `POST /api/v1/projects/{projectId}/env-vars/reveal`: CLI-only endpoint for authorized plaintext retrieval with mandatory audit event.
- `POST /api/v1/projects/{projectId}/env-vars/seed`: bulk ingest env keys from CLI payload with provider selection strategy.

## CLI Command Draft
- `dagryn env set KEY --value <v> --project <id> --environment <env> [--branch <name>] [--secret] [--provider db|aws|gcp|cloudflare]`
- `dagryn env set KEY --from-stdin --project <id> --environment <env> [--branch <name>] --secret`
- `dagryn env seed --file .env --project <id> --environment <env> [--branch <name>] --provider <p> --overwrite`
- `dagryn env list --project <id> [--environment <env>] [--branch <name>] [--show-source]`
- `dagryn env pull --project <id> --environment <env> [--branch <name>] --format dotenv --output .env.local --overwrite`
- `dagryn env inject --project <id> --environment <env> [--branch <name>] -- <command ...>`
- `dagryn env reveal KEY --project <id> --environment <env> [--branch <name>]` (permission-gated, audit-logged)
- `dagryn env rotate KEY --project <id> --environment <env> [--branch <name>]` and `dagryn env delete KEY --project <id> --environment <env> [--branch <name>]`

## Data Model Draft (SQL)
- `project_env_vars` (record metadata + resolution scope)
  - `id UUID PK`, `project_id UUID NOT NULL`, `key TEXT NOT NULL`, `value_type TEXT NOT NULL` (`plain|secret`)
  - `environment TEXT NULL`, `branch TEXT NULL`, `required BOOLEAN NOT NULL DEFAULT false`, `enabled BOOLEAN NOT NULL DEFAULT true`
  - `provider TEXT NOT NULL` (`db|aws_sm|gcp_sm|cloudflare`), `provider_ref TEXT NULL`, `secret_record_id UUID NULL`
  - `description TEXT NULL`, `created_by UUID`, `updated_by UUID`, `created_at`, `updated_at`, `deleted_at`
  - Unique index recommendation: `(project_id, key, COALESCE(environment,''), COALESCE(branch,''), deleted_at)` with active-row constraint.
- `secret_records` (encrypted material or provider metadata)
  - `id UUID PK`, `provider TEXT NOT NULL`, `ciphertext BYTEA NULL` (DB provider), `key_ref TEXT NULL`, `checksum TEXT NULL`
  - `version TEXT NULL`, `external_ref TEXT NULL`, `status TEXT NOT NULL` (`active|rotated|revoked`)
  - `created_at`, `updated_at`, `rotated_at`
- `env_audit_events` (recommended third table)
  - `id UUID PK`, `project_id UUID NOT NULL`, `env_var_id UUID NULL`, `actor_id UUID NULL`, `action TEXT NOT NULL`
  - `provider TEXT NULL`, `environment TEXT NULL`, `branch TEXT NULL`, `metadata JSONB`, `created_at`
  - Action examples: `create`, `update`, `delete`, `rotate`, `reveal`, `pull`, `resolve`, `seed`.

## Milestones and Ticket Plan
- **Milestone 1: Core registry + DB provider**
  - Add migrations for `project_env_vars`, `secret_records`, `env_audit_events`.
  - Implement API CRUD/resolve/seed and CLI set/list/pull/inject using DB provider.
  - Add runtime injection path and warning-only missing-required behavior.
- **Milestone 2: Settings UI + policy controls**
  - Add `Environment Variables` project settings section with filters and branch-aware views.
  - Add secret masking UX, metadata editing, required toggle, and rotate/delete flows.
  - Add role checks for manage/reveal/rotate and full audit event emission.
- **Milestone 3: Multi-provider GA**
  - Implement AWS Secrets Manager, Google Secret Manager, and Cloudflare adapters.
  - Add provider health checks, failure classification, and fallback guidance.
  - Add cross-provider migration command path (`--migrate-provider`).
- **Milestone 4: Hardening + rollout**
  - Add observability dashboards (resolve latency, provider failures, reveal usage).
  - Add tenancy/rate-limit safeguards and operational playbooks.
  - Launch with docs, examples, and migration assistant for task-level env users.
