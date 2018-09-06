# knative Personas

When discussing user actions, it is often helpful to [define specific
user roles](https://en.wikipedia.org/wiki/Persona_(user_experience)) who
might want to do the action.


## knative Build

We expect the build components of knative to be useful on their own,
as well as in conjunction with the compute components. 

### Developer

The developer personas for build are broader than the serverless
workloads that the knative compute product focuses on. Developers
expect to have build tools which integrate with their native language
tooling for managing dependencies and even detecting language and
runtime dependencies.

User stories:
* Start a build
* Read build logs

### Language operator / contributor

The language operators perform the work of integrating language
tooling into the knative build system. This role may work either
within a particular organization, or on behalf of a particular
language runtime.

User stories:
* Create a build image / build pack
* Enable build signing / provenance


## Contributors

Contributors are an important part of the knative project. As such, we
will also consider how various infrastructure encourages and enables
contributors to the project, as well as the impact on end-users.

* Hobbyist or newcomer
* Motivated user
* Corporate (employed) maintainer
* Consultant

User stories:
* Check out the code
* Build and run the code
* Run tests
* View test status
* Run performance tests

