# corakong
a [Kong](https://konghq.com/) plugin for [Coraza WAF](https://coraza.io/).

## the kubernetes CRD
Using Kubernetes CRD to keep the SecLang rules.

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: wafrules.waf.ik8s.ir
spec:
  conversion:
    strategy: None
  group: waf.ik8s.ir
  names:
    kind: WAFRule
    listKind: WAFRuleList
    plural: wafrules
    shortNames:
    - wafrule
    singular: wafrule
  scope: Namespaced
  versions:
  - name: v1alpha1
    schema:
      openAPIV3Schema:
        properties:
          spec:
            properties:
              rule:
                type: string
            type: object
        type: object
    served: true
    storage: true
```
## Contribute
Contributions are welcome!
