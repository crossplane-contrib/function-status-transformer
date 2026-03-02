# Basic Example

This example demonstrates the simplest use case for `function-status-transformer`: matching a single status condition from a composed resource and transforming it into a condition on the composite resource.

## What This Example Demonstrates

The composition defines a pipeline with `function-status-transformer`. It is configured to:
1. Look for a composed resource named `my-resource`.
2. Check if that resource has a condition of type `Synced` with a status of `False`.
3. If matched, it sets a new condition on the Composite Resource: `CustomReady: False` with reason `ResourceNotSynced`.

## Testing Locally

We provide an `observed.yaml` file that simulates a scenario where `my-resource` has failed to sync.

To see the function transform the status:

```shell
crossplane render xr.yaml composition.yaml functions.yaml -o observed.yaml
```

### Expected Output

In the rendered output, you should look at the Composite Resource (the `XR` kind) at the very top. You will see two conditions added by the function:

1. `CustomReady: False` (The custom condition we configured)
2. `StatusTransformationSuccess: True` (The function reporting that it ran successfully)

```yaml
status:
  conditions:
  - lastTransitionTime: "..."
    message: The resource failed to sync
    reason: ResourceNotSynced
    status: "False"
    type: CustomReady
  - lastTransitionTime: "..."
    reason: Available
    status: "True"
    type: StatusTransformationSuccess
```
