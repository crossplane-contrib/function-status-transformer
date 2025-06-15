# function-status-transformer

[![CI](https://github.com/crossplane-contrib/function-status-transformer/actions/workflows/ci.yml/badge.svg)](https://github.com/crossplane-contrib/function-status-transformer/actions/workflows/ci.yml)

- [Requirements](#requirements)
- [Usage](#usage)
  - [Basic Usage](#basic-usage)
  - [Using Regular Expressions to Capture Message Data](#using-regular-expressions-to-capture-message-data)
  - [Using Regular Expressions to Match Multiple Resources](#using-regular-expressions-to-match-multiple-resources)
  - [Condition Matching Wildcards](#condition-matching-wildcards)
  - [Matchers are ANDed](#matchers-are-anded)
  - [Overriding Conditions](#overriding-conditions)
  - [Matching the Composite Resource](#matching-the-composite-resource)
  - [Matching Missing Conditions](#matching-missing-conditions)
  - [Setting Default Conditions](#setting-default-conditions)
  - [Creating Events](#creating-events)
  - [Customizing Matching Behavior](#customizing-matching-behavior)
- [Determining the Status of the Function Itself](#determining-the-status-of-the-function-itself)
  - [Success](#success)
  - [Failure to Parse Input](#failure-to-parse-input)
  - [Failure to Match a Regular Expression](#failure-to-match-a-regular-expression)
  - [Failure to Set a Condition Message Template](#failure-to-set-a-condition-message-template)

## Requirements

This function requires Crossplane v1.17 or newer.

## Usage

Function Status Transformer allows you to create hooks that will trigger by
matching the status conditions of managed resources. Each hook can be configured
to set one or more status conditions on the composite resource and the claim.

### Basic Usage

Here is a basic usage example. The function will look for the
`cloudsql-instance` resource within the observed resource map. If that resource
matches the specified condition criteria, the condition in `setConditions` will
get set on both the composite resource and the claim.

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
spec:
  compositeTypeRef:
    apiVersion: your.api.group/v1alpha1
    kind: XYourCompositeKind
  mode: Pipeline
  pipeline:
  # Insert your pipeline steps here.
  - step: status-handler
    functionRef:
      name: function-status-transformer
    input:
      apiVersion: function-status-transformer.fn.crossplane.io/v1beta1
      kind: StatusTransformation
      statusConditionHooks:
      - matchers:
        - resources: 
          # Note that "name" refers to the resource's identifier within the
          # Composition and not the object's metadata.name. 
          # Related documentation:
          # https://docs.crossplane.io/latest/guides/function-patch-and-transform/#resource-templates
          - name: "cloudsql-instance"
          conditions:
          - type: Synced
            status: "False"
            reason: ReconcileError
            message: "failed to create the database: some internal error."
        setConditions:
        - target: CompositeAndClaim
          condition:
            type: DatabaseReady
            status: "False"
            reason: FailedToCreate
            message: "failed to create the database"
```

### Using Regular Expressions to Capture Message Data

You can use regular expressions to capture data from the status condition
message on the managed resource. The captured groups can then be inserted into
status condition and event messages on the composite resource and claim.

```yaml
apiVersion: function-status-transformer.fn.crossplane.io/v1beta1
kind: StatusTransformation
statusConditionHooks:
- matchers:
  - resources:
    - name: "cloudsql-instance"
    conditions:
    - type: Synced
      status: "False"
      reason: ReconcileError
      message: "failed to create the database: (?P<Error>.+)"
  setConditions:
  - target: CompositeAndClaim
    condition:
      type: DatabaseReady
      status: "False"
      reason: FailedToCreate
      message: "Encountered an error creating the database: {{ .Error }}"
  createEvents:
  - target: CompositeAndClaim
    event:
      type: Warning
      reason: FailedToCreate
      message: "Encountered an error creating the database: {{ .Error }}"
```

### Using Regular Expressions to Match Multiple Resources

You can use regular expressions in the `resourceKey`. This will allow you to
match multiple resources of a similar type. For instance, say you spin up
multiple instances of the same type and name them like `cloudsql-1`,
`cloudsql-2`, etc. You could write a single hook to handle all of these by using
a resource key of `cloudsql-\\d+`.

```yaml
apiVersion: function-status-transformer.fn.crossplane.io/v1beta1
kind: StatusTransformation
statusConditionHooks:
- matchers:
  - resources:
    - name: "cloudsql-\\d+"
    conditions:
    - type: Synced
      status: "False"
      reason: ReconcileError
```

### Condition Matching Wildcards

If you do not care about the particular value of a status condition that you are
matching against, you can leave it empty and it will act as a wildcard. The only
value that cannot be left empty is the `type`.

```yaml
apiVersion: function-status-transformer.fn.crossplane.io/v1beta1
kind: StatusTransformation
statusConditionHooks:
- matchers:
  - resources:
    - name: "cloudsql-\\d+"
    conditions:
      # The "reason" and "message" fields will be treated as wildcards.
    - type: Synced
      status: "False"
```

### Matchers are ANDed

When using multiple `matchers`, they must all match before `setConditions` will
be triggered.

```yaml
apiVersion: function-status-transformer.fn.crossplane.io/v1beta1
kind: StatusTransformation
statusConditionHooks:
# Both matchers must be true before the corresponding setConditions will
# be set
- matchers:
  - resources:
    - name: "cloudsql"
    conditions:
    - type: Synced
      status: "True"
  - resources:
    - name: "cloudsql"
    conditions:
    - type: Ready
      status: "True"
  setConditions: {...}
```

### Overriding Conditions

Hooks will be executed in order. By default a `setCondition` will not set a
condition that was previously set by function-status-transformer (Note: This
does not apply to conditions set by previous functions in the pipeline).
Condition uniqueness is determined by the `type`. To override a condition that
was already set, you can use `force`.

```yaml
apiVersion: function-status-transformer.fn.crossplane.io/v1beta1
kind: StatusTransformation
statusConditionHooks:
- setConditions:
    # This condition will be set as it is the first time the type is set.
  - target: CompositeAndClaim
    condition:
      type: DatabaseReady
      status: "True"
      reason: Example
- setConditions:
    # This condition will not be set as it has been set before and is not
    # forceful.
  - target: CompositeAndClaim
    condition:
      type: DatabaseReady
      status: "False"
      reason: FailedToCreate
      message: "Encountered an error creating the database: {{ .Error }}"
- setConditions:
    # This condition will be set as it is forceful.
  - target: CompositeAndClaim
    force: true
    condition:
      type: DatabaseReady
      status: "False"
      reason: FailedToCreate
      message: "Encountered an error creating the database: {{ .Error }}"
```

### Matching the Composite Resource

You can match against the composite resource. To do this, use
`includeCompositeAsResource` as seen below.

```yaml
apiVersion: function-status-transformer.fn.crossplane.io/v1beta1
kind: StatusTransformation
statusConditionHooks:
- matchers:
  - includeCompositeAsResource: true
    conditions:
    - type: Synced
      status: "False"
      reason: "SomeError"
```

### Matching Missing Conditions

You can match against missing conditions. To do this, use the default unknown
condition values.

```yaml
apiVersion: function-status-transformer.fn.crossplane.io/v1beta1
kind: StatusTransformation
statusConditionHooks:
- matchers:
  - resources:
    - name: "cloudsql"
    conditions:
      # These are the values seen when a condition does not exist.
    - type: Synced
      status: "Unknown"
      reason: ""
      message: ""
```

### Setting Default Conditions

If you want to set one or more conditions when no other hook has matched, you
can do this by placing a hook at the end and make sure the `setCondition`
`force` values are omitted or set to false. For matching, you can either use all
wildcard fields (see [Condition Matching
Wildcards](#condition-matching-wildcards)) or you can specify a type that will
never exist. The only requirement is that the `resourceKey` must match one or
more resources.

```yaml
apiVersion: function-status-transformer.fn.crossplane.io/v1beta1
kind: StatusTransformation
statusConditionHooks:
- matchers:
  - resourceKey: "cloudsql"
    condition:
      # This is a type that does not exist on the resource.
      type: ThisTypeDoesNotExist
      # These are the values seen when a condition does not exist.
      status: "Unknown"
      reason: ""
      message: ""
- setConditions:
  - target: CompositeAndClaim
    force: false
    condition:
      type: DatabaseReady
      status: "Unknown"
      reason: Unknown
```

### Creating Events

In addition to setting conditions, you can also create events for both the
composite resource and the claim. You should note that events should be created
sparingly, and will be limited by the behavior documented in
[5802](https://github.com/crossplane/crossplane/issues/5802)

To create events, use `createEvents` as seen below.

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
spec:
  compositeTypeRef:
    apiVersion: your.api.group/v1alpha1
    kind: XYourCompositeKind
  mode: Pipeline
  pipeline:
  # Insert your pipeline steps here.
  - step: status-handler
    functionRef:
      name: function-status-transformer
    input:
      apiVersion: function-status-transformer.fn.crossplane.io/v1beta1
      kind: StatusTransformation
      statusConditionHooks:
      - matchers:
        - resources:
          - name: "cloudsql-instance"
          conditions:
          - type: Synced
            status: "False"
            reason: ReconcileError
            message: "failed to create the database: some internal error."
        createEvents:
        - target: CompositeAndClaim
          event:
            type: Warning
            reason: FailedToCreate
            message: "failed to create the database"
```

### Customizing Matching Behavior

Any given matcher will first find all resources selected by `matcher.resources`.
It will then compare the status conditions of the resources against the status
conditions found in `matcher.conditions`. You can customize the comparison
behavior by setting `matcher.type`. The different match types and their
behaviors can be seen below.

- `AnyResourceMatchesAnyCondition` - Considered a match if any resource matches
  any condition. An example use case would be if you want to transform any error
  condition from one of your managed resources into an error condition on your
  composite and claim.
- `AnyResourceMatchesAllConditions` - Considered a match if any resources
  matches all conditions. An example use case would be if you want to check if
  any resource is synced but not ready. You could then communicate to the user
  that everything is valid and they just need to wait for resources to become
  ready.
- `AllResourcesMatchAnyCondition` - Considered a match if all resources match
  any condition. An example use case would be if you want to check that some
  resources match one of the many possible good states.
- `AllResourcesMatchAllConditions` (default) - Considered a match if all
  resources match all conditions. An example use case would be checking that all
  resources are both synced and ready. You could then let the user know that
  everything is ready to go.

## Determining the Status of the Function Itself

The status of this function can be found by viewing the
`StatusTransformationSuccess` status condition on the composite resource. The
function will use this status condition to communicate its own state and will
not emit fatal results. This means that the overall state of the claim and
composite resource will not be affected by this functions failure. See the
following sections on some common conditions you may encounter and what they
mean.

Notes:

- Any error encountered within a `statusConditionHook` will be logged, but only
  the last error will be present on the `StatusTransformationSuccess` condition.

### Success

If no failures are encountered, the `StatusTransformationSuccess` condition will
be set to `True` with a reason of `Available`.

```yaml
- lastTransitionTime: "2024-08-02T15:57:20Z"
  reason: Available
  status: "True"
  type: StatusTransformationSuccess
```

### Failure to Parse Input

If an invalid input is provided, the `StatusTransformationSuccess` condition
will be set to `False` with a reason of `InputFailure`. Note that no
`matchCondition` or `setCondition` will be evaluated.

```yaml
- lastTransitionTime: "2024-08-02T15:11:35Z"
  message: 'cannot get Function input from *v1beta1.RunFunctionRequest: cannot get
    function input *v1beta1.StatusTransformation from *v1beta1.RunFunctionRequest:
    cannot unmarshal JSON from *structpb.Struct into *v1beta1.StatusTransformation:
    json: cannot unmarshal Go value of type v1beta1.StatusTransformation: unknown
    name "statusConditionHookss"'
  reason: InputFailure
  status: "False"
  type: StatusTransformationSuccess
```

### Failure to Match a Regular Expression

If an invalid regular expression is provided in a `matchCondition` `message` or
`resourceKey`, the `StatusTransformationSuccess` condition will be set to
`False` with a reason of `MatchFailure`. Note that no further `matchCondition`
will be evaluated for corresponding `statusConditionHook` and the overall result
of matching for the hook will be a failure. All other `statusConditionHooks`
will attempt to be evaluated as normal.

```yaml
- lastTransitionTime: "2024-08-02T15:29:51Z"
  message: 'error when matching, statusConditionHookIndex: 0, matchConditionIndex:
    0: [failed to compile message regex, error parsing regexp: invalid or unsupported
    Perl syntax: `(?!`]'
  reason: MatchFailure
  status: "False"
  type: StatusTransformationSuccess
```

### Failure to Set a Condition Message Template

If an invalid template is provided in a `setCondition` message, the
`StatusTransformationSuccess` condition will be set to `False` with a reason of
`SetConditionFailure`.

```yaml
- lastTransitionTime: "2024-08-02T15:46:45Z"
  message: 'failed to set condition, statusConditionHookIndex: 0, setConditionIndex:
    0: [failed to parse template, template: :1: unexpected "}" in operand]'
  reason: SetConditionFailure
  status: "False"
  type: StatusTransformationSuccess
```
