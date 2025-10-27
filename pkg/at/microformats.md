# Microformats

## Background

- https://en.wikipedia.org/wiki/Microformat
- 2005 - RDF
- 'add semantics to a page' - html/markdown focus on how things looks.
- microformats.org
- uses 'class', `<a rel=../>`, rev
- example: `<span class="geo"><span class="latitude">1...`
    - hCard: tel, etc
    - hCalendar
    - rel-directory
    - rel-nofollow
    - rel-tag
    - XFN - relations

Alternative: RDFa, using attributes:
- about
- rel/rev
- src, href, resource
- property
- typeof

[Json-LD](https://json-ld.org/),
[Embedded-json-ld]

(https://w3c.github.io/json-ld-syntax/#embedding-json-ld-in-html-documents)
```
<script type="application/ld+json">
{
  "@context": "https://json-ld.org/contexts/person.jsonld",
  "@id": "http://dbpedia.org/resource/John_Lennon",
  "name": "John Lennon",
  "born": "1940-10-09",
  "spouse": "http://dbpedia.org/resource/Cynthia_Lennon"
}
</script>

```

- RDF - set of graphs
- graph = nodes connected by links (arcs, properties)
- 

```
{
  "@context": "https://json-ld.org/contexts/person.jsonld",
  "@id": "http://dbpedia.org/resource/John_Lennon",
  "name": "John Lennon",
  "born": "1940-10-09",
  "spouse": "http://dbpedia.org/resource/Cynthia_Lennon"
}
```

[microdata](https://html.spec.whatwg.org/multipage/microdata.html#microdata) appear most popular https://webdatacommons.org/structureddata/index.html#toc2

```html
<div itemscope>
 <p>My name is <span itemprop="name">Elizabeth</span>.</p>
</div>

<div itemscope>
 <p>My name is <span itemprop="name">Daniel</span>.</p>
</div>
<h1 itemscope>
 <data itemprop="product-id" value="9678AOU879">The Instigator 2000</data>
</h1>
<div itemscope>
 I was born on <time itemprop="birthday" datetime="2009-05-10">May 10th 2009</time>.
</div>
<meter itemprop="ratingValue" min=0 value=3.5 max=5>Rated 3.5/5</meter>
<section itemscope itemtype="https://example.org/animals#cat">
 <h1 itemprop="name">Hedral</h1>
 <p itemprop="desc">Hedral is a male american domestic
 shorthair, with a fluffy black fur with white paws and belly.</p>
 <img itemprop="img" src="hedral.jpeg" alt="" title="Hedral, age 18 months">
</section>

<dl itemscope
    itemtype="https://vocab.example.net/book"
    itemid="urn:isbn:0-330-34032-8">
 <dt>Title
 <dd itemprop="title">The Reality Dysfunction
 <dt>Author
 <dd itemprop="author">Peter F. Hamilton
 <dt>Publication date
 <dd><time itemprop="pubdate" datetime="1996-01-26">26 January 1996</time>
</dl>
```
- item = group of property name and values. Can have itemtype, itemid. A value can be an item.

# Markdown

Example:

[_metadata_:author]:- "daveying
[_metadata_:tags]:- "markdown metadata"

[author](foo):test "daveying"

[_metadata_:tags]:- "markdown metadata"

The "-" result in no rendering.

"Link reference" https://spec.commonmark.org/0.29/#link-reference-definitions

[Normal link](http://example.com) vs

[Link reference]: foo "bar"

- indented up to 3 spaces
- link label ([])
- Colon (:)
- space (including a line ending)
- link destination
- space 
- link title

[link](/uri "title")



# Hugo

https://gohugo.io/content-management/markdown-attributes/

Paragraph
{class="foo"}


# Diagrams {#my-diagrams}

https://mermaid.js.org/syntax/flowchart.html

# Anchors and links

On github - "My paragraph title" will produce the following anchor user-content-my-paragraph-title

[Link](#anchors-and-links)

Gitbook uses {#my-anchor}

Pandoc: [this is pookie]{#pookie} - creates a span with ID pookie.

[Some Text](){:name='anchorName'}


