# Events & Regex Extraction Example

This example demonstrates a more advanced feature of `function-status-transformer`: the ability to capture pieces of a message in a managed resource condition using Regular Expressions, and then template that captured text into new conditions and trigger Kubernetes Events!

## What This Example Demonstrates

The composition defines a pipeline with `function-status-transformer`. It is configured to:
1. Look for a composed resource named `my-database`.
2. Check if that resource has a `Synced: False` condition with a reason of `ReconcileError`.
3. Use a Regular Expression `message: "failed to create the database: (?P<Error>.+)"` to create a capture group called `Error`.
4. Define a `setCondition` targeting `CompositeAndClaim`, injecting `{{ .Error }}` into the new message.
5. Create a `Warning` event, also injecting the `{{ .Error }}` into the event content.

## Testing Locally

We provide an `observed.yaml` file that simulates a scenario where `my-database` failed to sync due to "Invalid credentials provided to the engine."

Run this to see the text extraction and event emission:

```shell
crossplane render xr.yaml composition.yaml functions.yaml -o observed.yaml --include-function-results
```

### Expected Output

When rendered, you will note the `CustomReady` condition contains the text "Invalid credentials provided to the engine." Additionally, check the `events` array that is produced against the XR to see the warning event.

```yaml
status:
  conditions:
  - lastTransitionTime: "..."
    message: 'Encountered an error creating the database: Invalid credentials provided
      to the engine.'
    reason: FailedToCreate
    status: "False"
    type: DatabaseReady
# ...
---
apiVersion: example.crossplane.io/v1
kind: XR
metadata:
  name: example-xr
# ...
---
action: ""
apiVersion: events.k8s.io/v1
deprecatedCount: 0
deprecatedFirstTimestamp: null
deprecatedLastTimestamp: null
deprecatedSource: {}
eventTime: null
kind: Event
metadata:
  creationTimestamp: null
note: 'Encountered an error creating the database: Invalid credentials provided to
  the engine.'
reason: FailedToCreate
# ...
```
