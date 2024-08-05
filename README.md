# function-status-transformer
[![CI](https://github.com/dalton-hill-0/function-status-transformer/actions/workflows/ci.yml/badge.svg)](https://github.com/dalton-hill-0/function-status-transformer/actions/workflows/ci.yml)

- [Requirements](#requirements)
- [Usage](#usage)
  - [Basic Usage](#basic-usage)
  - [Using Regular Expressions to Capture Message Data](#using-regular-expressions-to-capture-message-data)
  - [Using Regular Expressions to Match Multiple Resources](#using-regular-expressions-to-match-multiple-resources)
  - [Condition Matching Wildcards](#condition-matching-wildcards)
  - [MatchConditions are ANDed](#matchconditions-are-anded)
  - [Overriding Conditions](#overriding-conditions)
  - [Setting Default Conditions](#setting-default-conditions)
  - [Creating Events](#creating-events)
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
get set on both the composite resource and the claim. Additionally, the event in
`createEvents` will get created on the composite resource.
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
        - matchConditions:
          - resourceName: "cloudsql-instance"
            condition:
              type: Synced
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
          createEvents:
          - target: CompositeAndClaim
            event:
              type: Warning
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
- matchConditions:
  - resourceName: "cloudsql-instance"
    condition:
      type: Synced
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
You can use regular expressions in the `resourceName`. This will allow you to
match multiple resources of a similar type. For instance, say you spin up
multiple instances of the same type and name them like `cloudsql-1`,
`cloudsql-2`, etc. You could write a single hook to handle all of these by using
a resource name of `cloudsql-\\d+`.
```yaml
apiVersion: function-status-transformer.fn.crossplane.io/v1beta1
kind: StatusTransformation
statusConditionHooks:
- matchConditions:
  - resourceName: "cloudsql-\\d+"
    condition:
      type: Synced
      status: "False"
      reason: ReconcileError
```

When matching multiple resources with the same match condition, you can choose
whether to match all resources or at least one. This can be done by specifying
`matchCondition.Type`. The default behavior is to match all resources.
```yaml
apiVersion: function-status-transformer.fn.crossplane.io/v1beta1
kind: StatusTransformation
statusConditionHooks:
- matchConditions:
  - resourceName: "cloudsql-\\d+"
    # MatchAny: Only one resource matching the resourceName regex must match.
    # MatchAll: All of the resources matchin the resourceName regex must match.
    # Defaults to MatchAll.
    type: "MatchAny"
    condition:
      type: Synced
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
- matchConditions:
  - resourceName: "cloudsql-\\d+"
    condition:
      # This will treat "reason" and "message" as wildcards.
      type: Synced
      status: "False"
```

### MatchConditions are ANDed
When using multiple `matchConditions`, they must all match before
`setConditions` will be triggered.
```yaml
apiVersion: function-status-transformer.fn.crossplane.io/v1beta1
kind: StatusTransformation
statusConditionHooks:
# Both matchConditions must be true before the corresponding setConditions will
# be set
- matchConditions:
  - resourceName: "cloudsql"
    condition:
      type: Synced
      status: "True"
  - resourceName: "cloudsql"
    condition:
      type: Ready
      status: "True"
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

### Setting Default Conditions
You can set default conditions even when the observed resource has no status
conditions. To do this, match unknown conditions.
```yaml
apiVersion: function-status-transformer.fn.crossplane.io/v1beta1
kind: StatusTransformation
statusConditionHooks:
- matchConditions:
  - resourceName: "cloudsql"
    condition:
      type: Synced
      status: "Unknown"
```

You can also leave `matchConditions` empty, which will match everything. This
could be useful if you want to add a default hook at the end that will only set
conditions if they have not yet been set.
```yaml
apiVersion: function-status-transformer.fn.crossplane.io/v1beta1
kind: StatusTransformation
statusConditionHooks:
- matchConditions: []
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
        - matchConditions:
          - resourceName: "cloudsql-instance"
            condition:
              type: Synced
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
If no failures are encountered, the `StatusTransformationSuccess` condition will be
set to `True` with a reason of `Available`.
```yaml
- lastTransitionTime: "2024-08-02T15:57:20Z"
  reason: Available
  status: "True"
  type: StatusTransformationSuccess
```

### Failure to Parse Input
If an invalid input is provided, the `StatusTransformationSuccess` condition will be
set to `False` with a reason of `InputFailure`. Note that no `matchCondition` or
`setCondition` will be evaluated.
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
`resourceName`, the `StatusTransformationSuccess` condition will be set to
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
