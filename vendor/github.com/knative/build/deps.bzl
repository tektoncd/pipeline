load("@io_bazel_rules_docker//container:container.bzl", "container_pull")

def repositories():
  container_pull(
      name = "git_base",
      registry = "gcr.io",
      # The gcloud container has git, but doesn't install the gcloud.sh
      # cred helper which causes problems.
      repository = "cloud-builders/gcloud",
      tag = "latest",
  )
