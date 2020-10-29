{
  namespaced(obj, ns):: if std.isObject(obj) then
    {
      [k]: if std.isObject(obj[k]) then (
        if std.objectHas(obj[k], 'kind') && !std.startsWith(obj[k].kind, 'Cluster') then obj[k] {
          metadata+: {
            namespace: ns,
          },
        } else $.namespaced(obj[k], ns)
      ) else obj[k]
      for k in std.objectFields(obj)
    } else obj,
}
