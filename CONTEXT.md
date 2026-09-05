# Service Repository

This context defines the architectural language shared by every business area and process in a service repository created from this template.

## Language

**Service Repository**:
A standalone business codebase that contains shared business capabilities and one or more independently deployable Roles.
_Avoid_: Generated Project, scaffold output

**Role**:
An independently built and deployed process entry point, currently API, Job, or Consumer.
_Avoid_: Runtime mode, profile

**Feature**:
A bounded business capability containing its domain rules and application use cases and usable by one or more Roles.
_Avoid_: Technical module, controller package

**Capability**:
A reusable technical integration available to a Role, such as MySQL, Redis, RocketMQ, or upstream authentication.
_Avoid_: Feature, plugin

**Adapter**:
Infrastructure code that connects a Role or Feature to an external protocol, scheduler, message broker, or data store.
_Avoid_: Utility, business service

## Identity and access

**Account**:
A person's sign-in identity and profile inside this service.
_Avoid_: User entity, operator

**Session**:
A revocable authenticated relationship between an Account and one client, represented by an access token and a refresh token.
_Avoid_: Login record, JWT

**Role**:
A named assignment of Permissions and one Data Scope that can be granted to Accounts.
_Avoid_: User type, group

**Permission**:
A stable authorization key for one protected operation.
_Avoid_: Button, menu permission

**Data Scope**:
A Role rule that limits which Accounts are visible through organization ownership; multiple Role scopes combine as a union.
_Avoid_: SQL filter, department permission

**Menu**:
A navigation or action definition exposed to the administration client; a button Menu may carry a Permission.
_Avoid_: Route table, permission

**Department**:
A node in the organization hierarchy to which an Account may belong.
_Avoid_: Group, team

## Notices

**Notice**:
A durable announcement addressed either to every Account or to a specified set of Accounts.
_Avoid_: Push message, SSE event

**Draft Notice**:
An editable Notice that has not yet been made visible to its audience.
_Avoid_: Unpublished message

**Published Notice**:
A Notice visible to its audience; it is immutable and may only be revoked.
_Avoid_: Active draft

**Revoked Notice**:
A formerly published Notice that is no longer visible to its audience and cannot be republished.
_Avoid_: Draft, deleted notice

## Administration

**Operation Log**:
A best-effort record of an authenticated administration action, used for operational review and aggregate activity statistics.
_Avoid_: Page view, compliance audit trail
