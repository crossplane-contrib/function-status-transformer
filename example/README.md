# function-status-transformer Examples

This directory contains examples of how to use the `function-status-transformer` to automatically handle condition states on managed resources.

We provide simulated testing examples so you can learn without a live cluster. To run any of these examples locally, you need the Crossplane CLI (`crossplane render`) installed.

## Provided Examples

- [**Basic**](./basic/): The simplest use case. Matches a single resource condition and transforms it into a Composite Resource condition.
- [**Events**](./events/): Demonstrates how to use Regex capture groups to parse error strings on a Managed Resource and use the extracted text to emit a Custom Condition and a Kubernetes `Warning` Event on the Composite.
- [**Advanced**](./advanced/): Demonstrates complex evaluation. Matches multiple instances of a resource class (e.g. `cloudsql-\d+`) and uses advanced custom matching (`AnyResourceMatchesAnyCondition` and `AllResourcesMatchAllConditions`) to orchestrate a clustered status update.

## Developing Local Tests

Each example has its own `observed.yaml` file, simulating a production failure or in-progress provisioning state. You can learn how to debug the pipelines directly by reading the `README.md` in each individual directory.
