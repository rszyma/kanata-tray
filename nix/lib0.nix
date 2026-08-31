lib: rec {
  # copied from:
  # https://discourse.nixos.org/t/nix-function-to-merge-attributes-records-recursively-and-concatenate-arrays/2030/9
  deepMerge =
    lhs: rhs:
    lhs
    // rhs
    // (builtins.mapAttrs (
      rName: rValue:
      let
        lValue = lhs.${rName} or null;
      in
      if builtins.isAttrs lValue && builtins.isAttrs rValue then
        deepMerge lValue rValue
      else if builtins.isList lValue && builtins.isList rValue then
        lValue ++ rValue
      else
        rValue
    ) rhs);

  deepMergeList = attrsList: lib.foldl' deepMerge { } attrsList;
}
