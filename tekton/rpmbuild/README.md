Tekton CLI RPM Build
====================

This is a tekton task to run a rpm build on copr

The spec file is not really well made at the moment, it doesn't do a source
build but a binary build which is really that would raise a few eyebrows among the real packagers.
We are hoping to improve that in the future.

It only supports the latest release as releasd on github. It queries the github
api to get the latest one.

It uses the docker image from `quay.io/chmouel/rpmbuild`, the Dockerfile is in
this directory.

It uploads it to `https://copr.fedorainfracloud.org/coprs/chmouel/tektoncd-cli/`
to actually use it on your Linux machine you simply have to do (on a recentish distro) :

```
dnf copr enable chmouel/tektoncd-cli
```

USAGE
=====

Same as when you use the [release.pipeline.yaml](../release-pipeline.yml) you
need to have a PipelineResource for your git repository. See
[here](../release-pipeline-run.yml) for an example.

* You need to have your user added to the `https://copr.fedorainfracloud.org/coprs/chmouel/tektoncd-cli/` request it by goign to [here ](https://copr.fedorainfracloud.org/coprs/chmouel/tektoncd-cli/permissions/) and ask for admin access.

* You  need to get your API file from https://copr.fedorainfracloud.org/api/ and have it saved to `~/.config/copr`

* You create the secret from that copr config file :

```
kubectl create secret generic copr-cli-config --from-file=copr=${HOME}/.config/copr
```

* And then you should be able create the task with :

```
kubectl create -f rpmbuild.yml
```

and  run it with :

```
kubectl create -f rpmbuild-run.yml
```

* Use `tkn tr ls` to make sure it didn't fails on validation and

```
oc logs --all-containers=true $(oc get pod -l "tekton.dev/taskRun=rpmbuild-pipeline-run" -o name) --follow
```

to get the output
