# Micro Yaml

Yaml has a lot of problems - noyaml.com has a great summary. The spec has a lot of 
complexity and made some wrong choices, no question about it.

However, that doesn't mean using plain old json - or one of the dozens of other flawed
formats is the only solution.

The main idea in yaml is to use the indentation to represent structure, like python does. Everything else in the spec can be ignored - with some tools to convert from
yaml to json and json to 'clean yaml'.

Micro Yaml is just yaml to json to yaml, using a dumb parser.

Some spec changes are needed to further clean things up:
- only 'true' and 'false' for boolean values ( no 'no' and 'yes' )
- same number representations that js allows
- no tags, anchors, aliases, mapping keys, folded scalars (only literals)

# Alternatives


https://pypi.org/project/strictyaml/

