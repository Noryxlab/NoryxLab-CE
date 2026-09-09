#!/usr/bin/env python3
"""
Fills the OpenAPI document with the routes the platform actually serves.

The document described 40 paths while the router registered 181, and nothing
said so: a client reading it saw a fifth of the API and had no way to know. The
missing four fifths were not secret, only undocumented, which is the worse of
the two - it teaches people to read the code instead.

This does not replace hand-written entries. It reads the router, finds the
routes with no entry, and writes one for each: the method, the path parameters,
a summary derived from the handler's name, and the responses every endpoint on
this platform can return. Anything already in the document is left exactly as
it is, because a generated line is worth less than a written one and must never
overwrite it.

    python3 scripts/ops/generate-openapi.py [--check]

--check writes nothing and exits non-zero if a route is undocumented. That is
what CI runs, and it is the part that keeps this from drifting again.
"""

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
DOCUMENT = ROOT / "backend/internal/http/static/openapi.yaml"
PUBLIC_DOCUMENT = ROOT / "backend/internal/http/static/openapi.public.yaml"
ROUTE_FILES = [
    ROOT / "backend/internal/http/server.go",
    ROOT.parent / "NoryxLab-EE/overlay/backend/internal/http/ee_routes.go",
]

ROUTE = re.compile(r'mux\.HandleFunc\("(GET|POST|PUT|DELETE|PATCH) ([^"]+)",\s*([A-Za-z0-9_.]+)\)')

# Which family an endpoint belongs to, read from its path. Tags are how a
# reader finds anything in a document this size.
TAGS = [
    ("/api/v1/admin/backups", "Backups"),
    ("/api/v1/admin", "Admin"),
    ("/api/v1/projects", "Projects"),
    ("/api/v1/workspaces", "Workloads"),
    ("/api/v1/jobs", "Workloads"),
    ("/api/v1/builds", "Workloads"),
    ("/api/v1/apps", "Workloads"),
    ("/api/v1/dashboards", "Workloads"),
    ("/api/v1/cronjobs", "Workloads"),
    ("/api/v1/datasets", "Datasets"),
    ("/api/v1/datasources", "Datasets"),
    ("/api/v1/ontologies", "Datasets"),
    ("/api/v1/cohorts", "Datasets"),
    ("/api/v1/repositories", "Repositories"),
    ("/api/v1/secrets", "Secrets"),
    ("/api/v1/user", "Account"),
    ("/api/v1/auth", "Auth"),
    ("/api/v1/egress", "Governance"),
    ("/workspaces", "Proxies"),
    ("/apps", "Proxies"),
    ("/dashboards", "Proxies"),
]


# Which endpoints are the platform's own, and which ones an integrator may
# build on.
#
# Everything the platform serves is documented - that part does not change. But
# a document that makes no distinction turns the interface's own aggregations
# into a contract by accident: the day somebody scripts against
# `/api/v1/admin/overview` because Swagger described it, the console can no
# longer change the shape of its own dashboard.
#
# So each operation is stamped `x-noryx-internal: true` or left alone, and the
# public document is the same file with the internal operations removed. The
# rule for the list below: an endpoint is internal when only our interface or
# our operators call it, and the shape of its answer follows a screen rather
# than a domain object.
INTERNAL = [
    # Pages, not API.
    "= /",
    "= /api/",
    "/swagger",
    # The interface's own state and aggregations.
    "/api/v1/user/preferences",
    "/api/v1/search",
    "/api/v1/platform/overview",
    "/api/v1/admin/overview",
    "/api/v1/admin/inventory",
    "/api/v1/admin/rbac-matrix",
    "/api/v1/admin/data-usage",
    "/api/v1/admin/executions",
    "/api/v1/pods",
    # The assistant widget in the interface. The developer API beside it
    # (/api/v1/assistant/developer/...) is deliberately public: it is
    # OpenAI-compatible and exists to be called from outside.
    "= /api/v1/assistant/chat",
    # Proxies: the response is whatever the workload behind them returns.
    "/api/v1/projects/{projectID}/files",
    "/workspaces/",
    "/apps/",
    "/dashboards/",
]


def internal(path):
    for rule in INTERNAL:
        if rule.startswith("= "):
            if path == rule[2:]:
                return True
        elif path.startswith(rule):
            return True
    return False


# --- the answers the generic families do not cover -------------------------
#
# Most endpoints return a domain object and are described by FAMILIES above.
# The rest return an envelope assembled in the handler - `map[string]any` in
# Go, which has no type to read - or answer with no body at all. Those are
# declared here, once, and generated from this table: 42 supported operations
# used to answer `200 Success` and say nothing about the payload, which is a
# documented route and an undocumented contract.

# Structs to turn into schemas beside the ones the families already pull in.
EXTRA_SCHEMAS = [
    ("internal/domain/usage", {"Sample": "UsageSample", "Total": "UsageTotal"}),
    ("internal/domain/quota", {"Quota": "Quota", "Usage": "QuotaUsage"}),
    ("internal/iam/keycloak", {"SMTPSettings": "SMTPSettings"}),
    ("internal/domain/datasource", {"ServiceDefinition": "DatasourceDefinition"}),
]

# Envelopes, written out because Go builds them from a map. `$Name` is a
# reference to another schema, `[$Name]` an array of them.
ENVELOPES = {
    "HealthResponse": {"status": "string"},
    "ProjectTokenResponse": {"token": "$ApiToken", "secret": "string", "note": "string"},
    "DatasetUsage": {
        "objects": "integer",
        "totalBytes": "integer",
        "truncated": "boolean",
        "measuredAt": "string",
    },
    "CohortMembersResponse": {
        "cohort": "$Cohort",
        "members": "[$CohortMember]",
        "subjects": "[string]",
        "shown": "integer",
        "total": "integer",
    },
    "ProjectQuotaResponse": {"quota": "$Quota", "usage": "$QuotaUsage", "limited": "boolean"},
    "ProjectUsageResponse": {"total": "$UsageTotal", "samples": "[$UsageSample]"},
    "PlatformUsageResponse": {"from": "date-time", "to": "date-time", "items": "[$UsageTotal]"},
    "SMTPSettingsResponse": {"settings": "$SMTPSettings", "configured": "boolean"},
    "SMTPTestResponse": {"sent": "boolean", "recipient": "string"},
    "AdminEgressRuleListResponse": {"items": "[$EgressRule]", "profiles": "[string]", "enforced": "boolean"},
    "ProjectEgressRuleListResponse": {"items": "[$EgressRule]", "enforced": "boolean"},
    "DatasetAccessListResponse": {"items": "[$DatasetAccess]", "canManage": "boolean"},
    "ProjectOrganizationRole": {"organizationId": "string", "organizationName": "string", "role": "string"},
    "ProjectOrganizationRoleListResponse": {"items": "[$ProjectOrganizationRole]"},
    "DatasetDownloadURLResponse": {"url": "string", "filename": "string", "expiresAt": "date-time"},
    "AssistantModel": {"id": "string", "object": "string", "created": "integer", "owned_by": "string"},
    "AssistantModelListResponse": {"object": "string", "data": "[$AssistantModel]"},
    "BackupTargetStatus": {
        "configured": "boolean", "endpoint": "string", "bucket": "string",
        "prefix": "string", "region": "string", "updatedAt": "date-time",
    },
    "PlatformSetting": {
        "key": "string", "kind": "string", "label": "string", "description": "string",
        "values": "[string]", "fallback": "string", "value": "string",
        "source": "string", "overridable": "boolean",
    },
    "PlatformSettingListResponse": {"items": "[$PlatformSetting]"},
    "RBACPolicyRow": {
        "role": "string", "key": "string", "locked": "boolean", "description": "string",
        "project": "string", "dataset": "string", "ontology": "string",
        "datasource": "string", "environment": "string", "workload": "string",
        "governance": "string",
    },
    "RBACPolicyResponse": {
        "rows": "[$RBACPolicyRow]", "assignmentCounts": "object", "updatedAt": "date-time",
    },
    "DatasourceDefinitionListResponse": {"items": "[$DatasourceDefinition]"},
    "HardwareTierListResponse": {"items": "[$HardwareTier]"},
    "CronJobListResponse": {"items": "[$Job]"},
    "ProjectVariable": {
        "name": "string", "value": "string", "description": "string",
        "updatedBy": "string", "updatedAt": "date-time",
    },
    "ProjectVariableListResponse": {"items": "[$ProjectVariable]", "canReadValues": "boolean"},
    "BuildLogsResponse": {
        "buildId": "string", "jobName": "string", "projectId": "string",
        "status": "string", "podName": "string", "logs": "string",
        "pending": "boolean", "unavailable": "string",
    },
    "InvitationResponse": {"projectId": "string", "userId": "string", "role": "string", "status": "string"},
}

# What each operation answers with. A code other than 200 replaces the
# generated `200 Success`, which was wrong: four of these never return one.
RESPONSES = {
    ("GET", "/healthz"): "HealthResponse",
    ("GET", "/api/v1/hardware-tiers"): "HardwareTierListResponse",
    ("GET", "/api/v1/cronjobs"): "CronJobListResponse",
    ("GET", "/api/v1/datasource-definitions"): "DatasourceDefinitionListResponse",
    ("GET", "/api/v1/projects/{projectID}/quota"): "ProjectQuotaResponse",
    ("PUT", "/api/v1/admin/projects/{projectID}/quota"): "ProjectQuotaResponse",
    ("GET", "/api/v1/projects/{projectID}/usage"): "ProjectUsageResponse",
    ("GET", "/api/v1/admin/usage"): "PlatformUsageResponse",
    ("GET", "/api/v1/admin/smtp"): "SMTPSettingsResponse",
    ("PUT", "/api/v1/admin/smtp"): "SMTPSettingsResponse",
    ("POST", "/api/v1/admin/smtp/tests"): "SMTPTestResponse",
    ("GET", "/api/v1/admin/settings"): "PlatformSettingListResponse",
    ("PUT", "/api/v1/admin/settings/{key}"): "PlatformSettingListResponse",
    ("GET", "/api/v1/admin/rbac-policy"): "RBACPolicyResponse",
    ("PUT", "/api/v1/admin/rbac-policy"): "RBACPolicyResponse",
    ("GET", "/api/v1/admin/egress/rules"): "AdminEgressRuleListResponse",
    ("PUT", "/api/v1/admin/egress/rules/{egressRuleID}"): "EgressRule",
    ("GET", "/api/v1/projects/{projectID}/egress/rules"): "ProjectEgressRuleListResponse",
    ("GET", "/api/v1/admin/backups/config/status"): "BackupTargetStatus",
    ("PUT", "/api/v1/admin/backups/config"): "BackupTargetStatus",
    ("GET", "/api/v1/datasets/{datasetID}/access"): "DatasetAccessListResponse",
    ("PUT", "/api/v1/datasets/{datasetID}/access/{subjectType}/{subjectID}"): "DatasetAccess",
    ("PUT", "/api/v1/datasets/{datasetID}/access/{userID}"): "DatasetAccess",
    ("PUT", "/api/v1/datasets/{datasetID}"): "Dataset",
    ("PUT", "/api/v1/datasets/{datasetID}/ownership"): "Dataset",
    ("POST", "/api/v1/datasets/{datasetID}/download-url"): "DatasetDownloadURLResponse",
    ("GET", "/api/v1/projects/{projectID}/organization-roles"): "ProjectOrganizationRoleListResponse",
    ("PUT", "/api/v1/projects/{projectID}/organization-roles/{organizationID}"): "ProjectOrganizationRole",
    ("GET", "/api/v1/assistant/developer/v1/models"): "AssistantModelListResponse",
    ("GET", "/api/v1/user/organizations"): "OrganizationListResponse",
    ("GET", "/api/v1/projects/{projectID}/variables"): "ProjectVariableListResponse",
    ("GET", "/api/v1/builds/{buildID}/logs"): "BuildLogsResponse",
    ("PUT", "/api/v1/projects/{projectID}/variables/{name}"): "ProjectVariable",
}

# Operations that answer with something other than a JSON 200.
OTHER_RESPONSES = {
    ("DELETE", "/api/v1/builds/{buildID}"): ("204", "Deleted", None, None),
    ("DELETE", "/api/v1/environments/{environmentID}"): ("204", "Deleted", None, None),
    ("DELETE", "/api/v1/datasets/{datasetID}/access/{subjectType}/{subjectID}"): ("204", "Permission removed", None, None),
    ("DELETE", "/api/v1/datasets/{datasetID}/access/{userID}"): ("204", "Permission removed", None, None),
    ("DELETE", "/api/v1/projects/{projectID}/organization-roles/{organizationID}"): ("204", "Role removed", None, None),
    ("DELETE", "/api/v1/projects/{projectID}/variables/{name}"): ("204", "Variable removed", None, None),
    ("DELETE", "/api/v1/cohorts/{cohortID}"): ("204", "Cohort removed", None, None),
    ("DELETE", "/api/v1/projects/{projectID}/tokens/{tokenID}"): ("204", "Token revoked", None, None),
    ("POST", "/api/v1/projects/{projectID}/tokens"): ("201", "Token created; the secret is shown once", "application/json", "ProjectTokenResponse"),
    ("GET", "/api/v1/datasets/{datasetID}/usage"): ("200", "How much the dataset holds", "application/json", "DatasetUsage"),
    ("GET", "/api/v1/cohorts/{cohortID}/members"): ("200", "The files the cohort froze", "application/json", "CohortMembersResponse"),
    ("POST", "/api/v1/ontologies/{ontologyID}/cohorts"): ("201", "Cohort created", "application/json", "Cohort"),
    ("POST", "/api/v1/projects/{projectID}/invitations"): ("201", "Member invited", "application/json", "InvitationResponse"),
    ("POST", "/api/v1/dataservices"): ("201", "Data service created", "application/json", "Datasource"),
    ("GET", "/api/v1/auth/login"): ("200", "The sign-in page", "text/html", None),
    ("GET", "/api/v1/admin/usage.csv"): ("200", "Usage as CSV", "text/csv", None),
    # Passed through from the model gateway, in the OpenAI response shape.
    ("POST", "/api/v1/assistant/developer/v1/chat/completions"): ("200", "An OpenAI-compatible chat completion", "application/json", "object"),
    ("POST", "/api/v1/assistant/developer/v1/completions"): ("200", "An OpenAI-compatible completion", "application/json", "object"),
}


def routes():
    found = {}
    for path in ROUTE_FILES:
        if not path.exists():
            continue
        for method, route, handler in ROUTE.findall(path.read_text()):
            # `{path...}` is the router's wildcard; OpenAPI has no notion of one,
            # so it is documented as an ordinary parameter.
            found[(method, route.replace("...", ""))] = handler.split(".")[-1]
    return found


def documented(text):
    """(path, method) pairs already in the document, and where each path starts."""
    pairs, starts = set(), {}
    current = None
    in_paths = False
    for number, line in enumerate(text.splitlines()):
        if line.startswith("paths:"):
            in_paths = True
            continue
        if in_paths and line and not line.startswith(" "):
            in_paths = False
        if not in_paths:
            continue
        path_match = re.match(r"^  (/\S*):\s*$", line)
        if path_match:
            current = path_match.group(1)
            starts[current] = number
            continue
        method_match = re.match(r"^    (get|post|put|delete|patch):\s*$", line)
        if method_match and current:
            pairs.add((current, method_match.group(1).upper()))
    return pairs, starts


def summarise(handler, method, path):
    words = re.sub(r"(?<!^)(?=[A-Z])", " ", handler).strip()
    return words[0].upper() + words[1:] if words else f"{method} {path}"


def tag_for(path):
    for prefix, tag in TAGS:
        if path.startswith(prefix):
            return tag
    return "Platform"


def block(method, path, handler, indent="    "):
    parameters = re.findall(r"\{(\w+)\}", path)
    lines = [f"{indent}{method.lower()}:"]
    lines.append(f"{indent}  tags: [{tag_for(path)}]")
    lines.append(f"{indent}  summary: {summarise(handler, method, path)}")
    lines.append(f"{indent}  operationId: {handler[0].lower() + handler[1:]}")
    if internal(path):
        lines.append(f"{indent}  x-noryx-internal: true")
    lines.append(f"{indent}  security:")
    lines.append(f"{indent}    - bearerAuth: []")
    lines.append(f"{indent}    - xNoryxUser: []")
    if parameters:
        lines.append(f"{indent}  parameters:")
        for name in parameters:
            lines.append(f"{indent}    - name: {name}")
            lines.append(f"{indent}      in: path")
            lines.append(f"{indent}      required: true")
            lines.append(f"{indent}      schema: {{ type: string }}")
    lines.append(f"{indent}  responses:")
    lines.append(f"{indent}    '200':")
    lines.append(f"{indent}      description: Success")
    for code, ref in (("401", "Unauthorized"), ("403", "Forbidden"), ("404", "NotFound")):
        lines.append(f"{indent}    '{code}':")
        lines.append(f"{indent}      $ref: '#/components/responses/{ref}'")
    return lines


# --- schemas, read from the Go domain types ---------------------------------
#
# Hand-written schemas drift from the structs they describe, and a schema that
# lies is worse than a missing one: a client generates code from it. These are
# read from the domain package, so the document says what the API returns
# because it is the same source.

GO_TO_OPENAPI = {
    "string": ("string", None),
    "bool": ("boolean", None),
    "int": ("integer", None),
    "int64": ("integer", "int64"),
    "float64": ("number", None),
    "time.Time": ("string", "date-time"),
    "json.RawMessage": ("object", None),
}

STRUCT = re.compile(r"^type (\w+) struct \{(.*?)^\}", re.M | re.S)
FIELD = re.compile(r'^\s*(\w+)\s+([\[\]\*\w\.]+)\s+`[^`]*json:"([^"]+)"')


def go_schemas(package_dir, wanted):
    """Reads the named structs out of a Go package and returns OpenAPI schemas."""
    schemas = {}
    for source in sorted(Path(package_dir).glob("*.go")):
        if source.name.endswith("_test.go"):
            continue
        text = source.read_text()
        for name, body in STRUCT.findall(text):
            if name not in wanted:
                continue
            properties = []
            for _, go_type, tag in FIELD.findall(body):
                json_name = tag.split(",")[0]
                if json_name in ("", "-"):
                    continue
                properties.append((json_name, go_type))
            if properties:
                schemas[wanted[name]] = properties
    return schemas


def property_lines(go_type, indent):
    optional = go_type.startswith("*")
    go_type = go_type.lstrip("*")
    if go_type.startswith("[]"):
        inner = go_type[2:].lstrip("*")
        kind, fmt = GO_TO_OPENAPI.get(inner, ("string", None))
        lines = [f"{indent}type: array", f"{indent}items:", f"{indent}  type: {kind}"]
        if fmt:
            lines.append(f"{indent}    format: {fmt}")
        return lines
    if go_type.startswith("map["):
        return [f"{indent}type: object", f"{indent}additionalProperties: true"]
    kind, fmt = GO_TO_OPENAPI.get(go_type, ("string", None))
    lines = [f"{indent}type: {kind}"]
    if fmt:
        lines.append(f"{indent}format: {fmt}")
    if optional:
        lines.append(f"{indent}nullable: true")
    return lines


# What each endpoint family returns. The path prefix is the key because that is
# what a reader of the document has in front of them.
FAMILIES = [
    ("/api/v1/apps", "App", "internal/domain/app", {"App": "App"}),
    ("/api/v1/dashboards", "App", "internal/domain/app", {"App": "App"}),
    ("/api/v1/jobs", "Job", "internal/domain/job", {"Job": "Job"}),
    ("/api/v1/cronjobs", "Job", "internal/domain/job", {"Job": "Job"}),
    ("/api/v1/datasources", "Datasource", "internal/domain/datasource", {"Datasource": "Datasource"}),
    ("/api/v1/ontologies", "Ontology", "internal/domain/ontology", {"Ontology": "Ontology"}),
    ("/api/v1/cohorts", "Cohort", "internal/domain/cohort", {"Cohort": "Cohort", "Member": "CohortMember"}),
    ("/api/v1/ontologies/{ontologyID}/cohorts", "Cohort", "internal/domain/cohort", {"Cohort": "Cohort"}),
    ("/api/v1/user/api-tokens", "ApiToken", "internal/domain/apitoken", {"Token": "ApiToken"}),
    ("/api/v1/projects/{projectID}/tokens", "ApiToken", "internal/domain/apitoken", {"Token": "ApiToken"}),
    ("/api/v1/admin/hardware-tiers", "AdminHardwareTier", "internal/domain/hardware", {"Tier": "AdminHardwareTier"}),
    ("/api/v1/admin/backups/runs", "BackupRun", "internal/domain/backup", {"Run": "BackupRun"}),
    ("/api/v1/egress", "EgressRule", "internal/domain/egress", {"Rule": "EgressRule"}),
    ("/api/v1/admin/storage-endpoints", "StorageEndpoint", "internal/domain/storageendpoint", {"Endpoint": "StorageEndpoint"}),
    # Sub-resources of a project return the same objects as their top-level
    # families; a reader should not have to guess that.
    ("/api/v1/projects/{projectID}/datasources", "Datasource", "internal/domain/datasource", {"Datasource": "Datasource"}),
    ("/api/v1/projects/{projectID}/ontologies", "Ontology", "internal/domain/ontology", {"Ontology": "Ontology"}),
    ("/api/v1/projects/{projectID}/ontology", "Ontology", "internal/domain/ontology", {"Ontology": "Ontology"}),
    ("/api/v1/projects/{projectID}/workspaces", "Workspace", "internal/domain/workspace", {"Workspace": "Workspace"}),
    ("/api/v1/projects/{projectID}/jobs", "Job", "internal/domain/job", {"Job": "Job"}),
    ("/api/v1/projects/{projectID}/apps", "App", "internal/domain/app", {"App": "App"}),
    ("/api/v1/projects/{projectID}/builds", "Build", "internal/domain/build", {"Build": "Build"}),
    ("/api/v1/projects/{projectID}/datasets", "Dataset", "internal/domain/dataset", {"Dataset": "Dataset"}),
    ("/api/v1/admin/organizations", "Organization", "internal/iam/keycloak", {"Organization": "Organization"}),
    ("/api/v1/admin/users", "PlatformUser", "internal/iam/keycloak", {"User": "PlatformUser"}),
    ("/api/v1/organizations", "Organization", "internal/iam/keycloak", {"Organization": "Organization"}),
    ("/api/v1/pods", "PodLaunch", "internal/domain/pod", {"Pod": "PodLaunch"}),
    ("/api/v1/admin/health", "HealthEvent", "internal/domain/health", {"Event": "HealthEvent"}),
    ("/api/v1/admin/audit", "AuditEvent", "internal/domain/audit", {"Event": "AuditEvent"}),
    # The compliance surface: a customer's officer reads this one, so it says
    # what it returns rather than "Success".
    ("/api/v1/admin/software-inventory", "SoftwareInventory", "internal/inventory", {"Document": "SoftwareInventory", "Item": "SoftwareInventoryItem"}),
    ("/api/v1/production/apps", "App", "internal/domain/app", {"App": "App"}),
]


def schema_blocks():
    """The schemas the families need, read from the Go structs."""
    emitted, blocks = set(), []
    for _, schema_name, package, wanted in FAMILIES:
        if schema_name in emitted:
            continue
        found = go_schemas(ROOT / "backend" / package, wanted)
        for name, properties in found.items():
            if name in emitted:
                continue
            emitted.add(name)
            blocks.append(f"    {name}:")
            blocks.append("      type: object")
            blocks.append("      properties:")
            for json_name, go_type in properties:
                blocks.append(f"        {json_name}:")
                blocks.extend(property_lines(go_type, "          "))
            blocks.append(f"    {name}ListResponse:")
            blocks.append("      type: object")
            blocks.append("      properties:")
            blocks.append("        items:")
            blocks.append("          type: array")
            blocks.append("          items:")
            blocks.append(f"            $ref: '#/components/schemas/{name}'")
    return blocks


METHOD_LINE = re.compile(r"^    (get|post|put|delete|patch):\s*$")
PATH_LINE = re.compile(r"^  (/\S*):\s*$")


def operations(lines):
    """Yields (path, method, start, end) for every operation in the document."""
    current_path, start, method = None, None, None
    in_paths = False
    for number, line in enumerate(lines):
        if line.startswith("paths:"):
            in_paths = True
            continue
        if in_paths and line and not line.startswith(" "):
            in_paths = False
        boundary = (not in_paths) or PATH_LINE.match(line) or METHOD_LINE.match(line)
        if boundary and start is not None:
            yield current_path, method, start, number
            start, method = None, None
        if not in_paths:
            continue
        path_match = PATH_LINE.match(line)
        if path_match:
            current_path = path_match.group(1)
            continue
        method_match = METHOD_LINE.match(line)
        if method_match and current_path:
            method, start = method_match.group(1), number
    if start is not None:
        yield current_path, method, start, len(lines)


def mark_internal(lines):
    """Stamps `x-noryx-internal: true` on the operations only we call."""
    marked = list(lines)
    # Bottom up, so the line numbers of the operations above stay valid.
    for path, _, start, end in reversed(list(operations(lines))):
        body = marked[start + 1 : end]
        body = [line for line in body if line.strip() != "x-noryx-internal: true"]
        if internal(path):
            body.insert(0, "      x-noryx-internal: true")
        marked[start + 1 : end] = body
    return marked


def public_document(lines):
    """The same document with the internal operations removed."""
    kept = list(lines)
    for path, _, start, end in reversed(list(operations(lines))):
        if internal(path):
            kept[start:end] = []
    # A path whose every operation was internal would be left as an empty
    # mapping key, which is not a valid path item.
    out, number = [], 0
    while number < len(kept):
        line = kept[number]
        path_match = PATH_LINE.match(line)
        following = kept[number + 1] if number + 1 < len(kept) else ""
        if path_match and (not following.startswith("    ") or PATH_LINE.match(following)):
            number += 1
            continue
        out.append(line)
        number += 1
    header = (
        "# The supported Noryx API: every endpoint an integration may build on.\n"
        "# Generated from openapi.yaml by scripts/ops/generate-openapi.py - the\n"
        "# operations marked x-noryx-internal are the interface's own and are not\n"
        "# here, because they change with the screens that use them.\n"
    )
    return header + "\n".join(out) + "\n"


def write(lines):
    DOCUMENT.write_text("\n".join(lines) + "\n")
    PUBLIC_DOCUMENT.write_text(public_document(lines))



def envelope_blocks():
    """The declared envelopes, as schema blocks under components/schemas."""
    lines = []
    for name, properties in ENVELOPES.items():
        lines.append(f"    {name}:")
        lines.append("      type: object")
        lines.append("      properties:")
        for field, kind in properties.items():
            if kind.startswith("[$"):
                lines.append(f"        {field}:")
                lines.append("          type: array")
                lines.append(f"          items: {{ $ref: '#/components/schemas/{kind[2:-1]}' }}")
            elif kind == "[string]":
                lines.append(f"        {field}:")
                lines.append("          type: array")
                lines.append("          items: { type: string }")
            elif kind.startswith("$"):
                lines.append(f"        {field}: {{ $ref: '#/components/schemas/{kind[1:]}' }}")
            elif kind == "date-time":
                lines.append(f"        {field}: {{ type: string, format: date-time }}")
            else:
                lines.append(f"        {field}: {{ type: {kind} }}")
    return lines


def apply_declared_responses(lines):
    """Replaces the generated `200 Success` where the answer is declared."""
    out = list(lines)
    for path, method, start, end in reversed(list(operations(lines))):
        key = (method.upper(), path)
        schema = RESPONSES.get(key)
        other = OTHER_RESPONSES.get(key)
        if schema is None and other is None:
            continue
        # The block to replace: the '200' entry and its description, whatever
        # the generator or a human wrote there.
        for number in range(start, end):
            if out[number].strip() != "'200':":
                continue
            indent = out[number][: len(out[number]) - len(out[number].lstrip())]
            stop = number + 1
            while stop < end and out[stop].startswith(indent + " "):
                stop += 1
            if other is not None:
                code, description, media, ref = other
                block = [f"{indent}'{code}':", f"{indent}  description: {description}"]
                if media:
                    block.append(f"{indent}  content:")
                    block.append(f"{indent}    {media}:")
                    if ref == "object":
                        block.append(f"{indent}      schema: {{ type: object }}")
                    elif ref:
                        block.append(f"{indent}      schema:")
                        block.append(f"{indent}        $ref: '#/components/schemas/{ref}'")
                    else:
                        block.append(f"{indent}      schema: {{ type: string }}")
            else:
                block = [
                    f"{indent}'200':",
                    f"{indent}  description: Success",
                    f"{indent}  content:",
                    f"{indent}    application/json:",
                    f"{indent}      schema:",
                    f"{indent}        $ref: '#/components/schemas/{schema}'",
                ]
            out[number:stop] = block
            break
    return out



def main():
    check_only = "--check" in sys.argv
    text = DOCUMENT.read_text()
    pairs, starts = documented(text)
    registered = routes()

    missing = sorted(
        (path, method, handler)
        for (method, path), handler in registered.items()
        if (path, method) not in pairs
    )
    if not missing and not check_only:
        # Nothing to add, but the schemas and the response bodies are still
        # regenerated: they drift from the Go types on their own.
        lines = attach_schemas(text.splitlines())
        lines = insert_schemas(lines)
        write(mark_internal(apply_declared_responses(lines)))
        print(f"  every one of the {len(registered)} registered routes is documented; schemas refreshed")
        return 0
    if not missing:
        # Documented is not the same as current: the marking and the public
        # document are derived from this file and drift the moment somebody
        # edits it by hand. CI has to see that, or the public contract quietly
        # stops matching the one the platform serves.
        lines = mark_internal(apply_declared_responses(insert_schemas(attach_schemas(text.splitlines()))))
        stale = []
        if "\n".join(lines) + "\n" != text:
            stale.append(DOCUMENT.name)
        if not PUBLIC_DOCUMENT.exists() or public_document(lines) != PUBLIC_DOCUMENT.read_text():
            stale.append(PUBLIC_DOCUMENT.name)
        if stale:
            print(f"  {' and '.join(stale)} out of date; run scripts/ops/generate-openapi.py")
            return 1
        print(f"  every one of the {len(registered)} registered routes is documented")
        return 0
    if check_only:
        print(f"  {len(missing)} route(s) the router serves and the document does not describe:")
        for path, method, _ in missing[:20]:
            print(f"    {method} {path}")
        if len(missing) > 20:
            print(f"    ... and {len(missing) - 20} more")
        print("  run scripts/ops/generate-openapi.py to add them, then write real summaries.")
        return 1

    lines = text.splitlines()
    # Group by path so a new path is written once with all of its methods.
    by_path = {}
    for path, method, handler in missing:
        by_path.setdefault(path, []).append((method, handler))

    additions = []
    for path in sorted(by_path):
        if path in starts:
            continue  # handled below, inserted into the existing block
        additions.append(f"  {path}:")
        for method, handler in sorted(by_path[path]):
            additions.extend(block(method, path, handler))

    # Insert methods into paths that already exist, from the bottom up so the
    # line numbers above stay valid.
    for path in sorted(by_path, key=lambda p: -starts.get(p, -1)):
        if path not in starts:
            continue
        insert_at = starts[path] + 1
        payload = []
        for method, handler in sorted(by_path[path]):
            payload.extend(block(method, path, handler))
        lines[insert_at:insert_at] = payload

    end = len(lines)
    for number in range(len(lines) - 1, -1, -1):
        if lines[number].strip() and not lines[number].startswith(" "):
            if not lines[number].startswith("paths:"):
                end = number
            break
    lines[end:end] = additions

    lines = attach_schemas(lines)
    lines = insert_schemas(lines)
    write(mark_internal(apply_declared_responses(lines)))
    print(f"  documented {len(missing)} route(s); write real summaries for the ones that matter")
    return 0


def insert_schemas(lines):
    """Adds the generated schemas under components/schemas, once."""
    blocks = [block for block in schema_blocks() if block.strip()]
    for package, wanted in EXTRA_SCHEMAS:
        for name, properties in go_schemas(ROOT / "backend" / package, wanted).items():
            blocks.append(f"    {name}:")
            blocks.append("      type: object")
            blocks.append("      properties:")
            for json_name, go_type in properties:
                blocks.append(f"        {json_name}:")
                blocks.extend(property_lines(go_type, "          "))
    blocks.extend(envelope_blocks())
    if not blocks:
        return lines
    existing = set()
    for line in lines:
        match = re.match(r"^    (\w+):\s*$", line)
        if match:
            existing.add(match.group(1))
    filtered, skip = [], False
    for block in blocks:
        name = re.match(r"^    (\w+):\s*$", block)
        if name:
            skip = name.group(1) in existing
        if not skip:
            filtered.append(block)
    if not filtered:
        return lines
    for number, line in enumerate(lines):
        # `schemas:` sits at two spaces under `components:`; its entries at
        # four. Matching the wrong indent silently inserted nothing and left
        # every generated $ref pointing at a schema that did not exist - a
        # document that parses and breaks Swagger UI on load.
        if re.match(r"^  schemas:\s*$", line):
            return lines[: number + 1] + filtered + lines[number + 1 :]
    raise SystemExit("components.schemas not found in the document")


def attach_schemas(lines):
    """Points each family's 200 response at the schema it returns."""
    out = []
    current_path = None
    for number, line in enumerate(lines):
        path_match = re.match(r"^  (/\S*):\s*$", line)
        if path_match:
            current_path = path_match.group(1)
        out.append(line)
        if not line.strip() == "description: Success" or current_path is None:
            continue
        # Idempotence, learned the hard way: running the generator twice added
        # a second `content:` under the same response, and YAML rejects a
        # duplicate key - the document stopped parsing entirely.
        following = lines[number + 1] if number + 1 < len(lines) else ""
        if following.strip().startswith("content:"):
            continue
        schema = None
        for prefix, name, _, _ in FAMILIES:
            if current_path.startswith(prefix):
                schema = name
                break
        if schema is None:
            continue
        # A path ending in a parameter returns one object; otherwise a list.
        listed = not current_path.rstrip("/").endswith("}")
        target = f"{schema}ListResponse" if listed else schema
        indent = line[: len(line) - len(line.lstrip())]
        out.append(f"{indent}content:")
        out.append(f"{indent}  application/json:")
        out.append(f"{indent}    schema:")
        out.append(f"{indent}      $ref: '#/components/schemas/{target}'")
    return out


if __name__ == "__main__":
    raise SystemExit(main())
