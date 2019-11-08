# TektonCD CLI release process

- You need to be a tektoncd-cli [OWNERS](OWNERS) to be able to push the new branch

- You need to have your own tekton cluster, under `minikube`, `gcp` or other means.

- Start reading this [README](tekton/README.md) and you can start a new release
  with the [release.sh](tekton/release.sh) script.

- Update the main [README.md](README.md) and change the version number in there (i.e: `v0.5.0`)
  to the version you are releasing. Make a PR and get it merged ASAP.

- When release is done in https://github.com/tektoncd/cli/releases edit the
  release and change it from pre-release as released.

- When it's released you can build the rpm according to this
  [README](tekton/rpmbuild/README.md). Make sure you have done the PREREQ to be
  able to upload it to the copr repository. This could take some time if copr
  builder is busy.

- you need to edit the Changelog according to the other templates in there,
  usually you may want to do this in hackmd so you can have other cli
  maintainers do this with you. You can see an example
  [here](https://gist.github.com/chmouel/8a837af3a592df47db9e81da8846c673).

- Get some OSX person to test the `brew install tektoncd/cli/tektoncd-cli`

- Get some Fedora person to test the RPM :

```shell
dnf makecache
dnf upgrade tektoncd-cli
```

- Make an update for [homebrew-core](https://github.com/Homebrew/homebrew-core/blob/master/Formula/tektoncd-cli.rb) for tektoncd-cli formula, make sure you get the proper `sha256sum` and update it like this :

  https://github.com/Homebrew/homebrew-core/pull/46492

- Announce and spread the love on twitter, make sure you tag
  [@tektoncd](https://twitter.com/tektoncd) account so you get retweeted and
  perhaps add the major new features in the tweet. See [here](https://twitter.com/chmouel/status/1177172542144036869) for an example.
  Do not fear to add a bunch of  emojis 🎉🥳. Ask @vdemeester for tips 🤣.

- Notify the cli channel on slack.
