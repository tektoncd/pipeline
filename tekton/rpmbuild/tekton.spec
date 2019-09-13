%define version _VERSION_
%define debug_package %{nil}

Name: tektoncd-cli
Summary: Tekton CLI - The command line interface for interacting with Tekton
Version: %{version}
Release: 1
License: ASL 2.0
URL: https://github.com/tektoncd/cli
Source0: https://github.com/tektoncd/cli/archive/tkn_%{version}_Linux_x86_64.tar.gz

%description
The Tekton Pipelines cli project provides a CLI for interacting with Tekton!

More information about Tekton http://tekton.dev

%prep
%autosetup -c -n tkn-%{version}_Linux_x86_64

%install
install -d %{buildroot}%{_bindir}
install -p -m 0755 tkn %{buildroot}%{_bindir}/tkn

install -d %{buildroot}%{_datadir}/zsh/site-functions
./tkn completion zsh > %{buildroot}%{_datadir}/zsh/site-functions/_tkn

install -d %{buildroot}%{_datadir}/bash-completion/completions
./tkn completion bash > %{buildroot}%{_datadir}/bash-completion/completions/_tkn

%clean
rm -rf $RPM_BUILD_ROOT

%files
%defattr(-,root,root,-)
%doc *.md LICENSE
%{_bindir}/tkn
%{_datadir}/zsh/site-functions/_tkn
%{_datadir}/bash-completion/completions/_tkn

%changelog
* Fri Sep 13 2019 Chmouel Boudjnah <chmouel@redhat.com> -
- Initial build.
