# Local K8s dev loop (Kind cluster), infra + observability only - no
# MatchFlow services exist yet (see docs/ROADMAP.md). This is the
# production-parity alternative to docker-compose.dev.yml: same Redis/
# Postgres, run as Helm-chart-managed pods instead of plain containers.
#
# Usage: `make dev-k8s` creates the Kind cluster and runs `tilt up`.
#
# ── Grouping resources in the Tilt UI ──────────────────────────────
# `labels=["<group>"]` on any resource (helm_resource, k8s_resource,
# local_resource, ...) puts it in a collapsible section of that name in
# Tilt's sidebar - purely a UI grouping, it has no effect on build/apply
# order. Everything below is "infra": once real services exist (Phase 3,
# ROADMAP.md), give them their own `labels=["services"]` (or one label
# per service) so infra and application code stay visually separate as
# the Tiltfile grows.
#
# ── Ordering resources ──────────────────────────────────────────────
# `resource_deps=[...]` is the actual dependency graph - it controls
# build/apply order (a resource waits for its deps to be up), unrelated
# to labels. Both redis and postgresql declare `resource_deps=["bitnami"]`
# below so Tilt adds the Helm repo before trying to install anything
# from it. When a real service needs Redis/Postgres to exist first, give
# it `resource_deps=["redis", "postgresql"]`.

load("ext://helm_resource", "helm_resource", "helm_repo")

INFRA_LABELS = ["infra"]

helm_repo("bitnami", "https://charts.bitnami.com/bitnami", labels=INFRA_LABELS)

helm_resource(
    "redis",
    "bitnami/redis",
    flags=["--values=k8s/local/helm-values/redis.yaml"],
    resource_deps=["bitnami"],
    port_forwards=["6379:6379"],
    # flags= isn't tracked for changes on its own - deps= is required so
    # Tilt re-syncs when the values file is edited, not just on Tiltfile
    # changes.
    deps=["k8s/local/helm-values/redis.yaml"],
    labels=INFRA_LABELS,
)

helm_resource(
    "postgresql",
    "bitnami/postgresql",
    flags=["--values=k8s/local/helm-values/postgresql.yaml"],
    resource_deps=["bitnami"],
    port_forwards=["5432:5432"],
    deps=["k8s/local/helm-values/postgresql.yaml"],
    labels=INFRA_LABELS,
)

k8s_yaml("k8s/local/otel-lgtm.yaml")
k8s_resource(
    "otel-lgtm",
    port_forwards=["3000:3000", "4317:4317", "4318:4318", "4040:4040"],
    labels=INFRA_LABELS,
)
