# API Changes

✋ Before reading this, please read the
[API Compatibility Policy](../../api_compatibility_policy.md)

## CustomRuns

Before adding an optional field to the spec or status of `CustomRun`,
consider whether it could be part of the custom spec or custom status instead.
New fields should be added to the spec or status of the `CustomRun` API only if they
make sense for all custom run controllers to support.

## Deprecations

The following are things developers should keep in mind for deprecations. These
tips may not apply in every case, so use your best judgement! If you're not sure
how these affect your deprecation or timeline, ask the maintainers.

1. Assume default values are used.

   Feature flags are an excellent way for users to opt-in and try out new
   behavior before it goes live. However unless someone is trying to experiment
   with the new feature, many users and other tools built on top of the API
   won't know about it until it becomes the new default.

   Try to give users as much time with a new feature flag in opt-out mode as you
   can. Users may not update releases immediately, so this gives you more time
   to get feedback from a broader set of users while still having an escape
   hatch to revert back to old behavior if problems come up. This may mean
   splitting up your deprecation window into opt-in and opt-out phases. For
   particularly disruptive changes, consider making the switch to opt-out its
   own deprecation notification.

   A conservative deprecation sequence to move from field A -> B might look
   like:

   1. (inital state) Return field A
   2. Introduce new field B with opt-in feature flag (disabled by default).
   3. Set feature flag to return both A and B in responses by default.
   4. Set feature flag to opt-in users by default and only return field B.
   5. Remove feature flag.

2. Use Godoc `Deprecated:` comments.

   Go has a feature for using special comments to notify language servers / IDEs
   that types and fields are deprecated. Use this as much as possible!

   ```go
   // Foo is a ...
   //
   // Deprecated: use Bar instead
   type Foo struct {...}
   ```

   See https://github.com/golang/go/wiki/Deprecated for more details.

3. Tombstone API fields before removal.

   Removing fields from the API is most disruptive since it affects both the
   server and clients, but **clients are not guaranteed to run at the same
   version as the server**. This can break an integration's ability to safely
   talk to older API servers.

   A question to ask yourself is: "if I remove this field, can this client still
   be used with the oldest supported LTS server?". If the answer is no, removing
   the field is effectively creating new minimum compatibility version for the
   client. This means downstream integrations either need to accept the new
   minimum version, fork the type, or refuse to upgrade (none of which are
   particularly great).

   When deprecating a field, try to remove server functionality before removing
   support from the client. While servers do not need to populate or otherwise
   support the field, clients can still use it to communicate with older server
   versions. Once you are reasonably confident that the field is no longer used
   by supported servers (rule of thumb: target currently supported LTS servers),
   it can safely be removed from client types.

4. Maintain backwards compatibility of Go libraries when possible.

   [Keeping Your Modules Compatible](https://go.dev/blog/module-compatibility)
   offers some helpful suggestions.
