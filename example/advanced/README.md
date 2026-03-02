# Advanced Custom Matching Example

This example demonstrates how to use the *Matchers are ANDed*, *Resource Name Regex*, and *Custom Matching behavior* (`type: AnyResourceMatchesAnyCondition` / `type: AllResourcesMatchAllConditions`) to implement complex multi-resource transformation logic.

## What This Example Demonstrates

The composition defines an array of hooks that evaluate multiple resources at once. It uses the regex `cloudsql-\d+` to match any resource belonging to a theoretical fleet of instances (e.g. `cloudsql-1`, `cloudsql-2`).

There are two major matchers in the composition:

1. **AnyResourceMatchesAnyCondition**: This hook examines all matched resources. If *any* of the subset (even just one instance) fails to sync (`Synced: False`), it will trigger and publish `GlobalSyncError: True` on the Composite.
2. **AllResourcesMatchAllConditions**: This hook also examines all instances. It will exclusively fire if *every single one of them* has `Ready: True` AND `Synced: True`. If so, it publishes `CustomReady: True` on the Composite.

> Note: Matcher `type` defaults to `AllResourcesMatchAllConditions` if not explicitly specified. We have explicitly specified `type` in this example for maximum clarity.

## Testing Locally

We provide an `observed.yaml` file that simulates a scenario with *two* CloudSQL instances.
- `cloudsql-1` is perfectly healthy (Synced: True, Ready: True).
- `cloudsql-2` has stalled (Synced: False, Ready: Unknown).

Run this to see the complex hook logic evaluate:

```shell
crossplane render xr.yaml composition.yaml functions.yaml -o observed.yaml
```

### Expected Output

Because `cloudsql-2` failed to sync, the first hook (`AnyResourceMatchesAnyCondition`) will trigger.
Because `cloudsql-2` is not both `Ready` and `Synced`, the second hook (`AllResourcesMatchAllConditions`) will **not** trigger.

In the rendered output for the Composite Resource (`kind: XR`):

```yaml
status:
  conditions:
  - lastTransitionTime: "..."
    message: At least one CloudSQL instance failed to sync
    reason: OneOrMoreFailed
    status: "True"
    type: GlobalSyncError
  - lastTransitionTime: "..."
    reason: Available
    status: "True"
    type: StatusTransformationSuccess
```
