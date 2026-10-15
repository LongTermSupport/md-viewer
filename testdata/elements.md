# Heading level 1

A paragraph with *emphasis*, **strong**, ***both***, ~~strikethrough~~, `inline code`, a
[relative link](other.md), an [external link](https://example.com/), an autolink
<https://example.org/>, a bare link https://example.net/ and an email <someone@example.com>.
This line ends with a hard break\
and continues on the next line.

## Heading level 2

### Heading level 3

#### Heading level 4

##### Heading level 5

###### Heading level 6

> A block quote with **strong** text.
>
> > A nested quote.

- Unordered item one
- Unordered item two
  - Nested item
    - Nested again
- Unordered item three

1. Ordered item one
2. Ordered item two
   1. Nested ordered
      1. Nested ordered again

- [ ] An open task
- [x] A done task

Loose list:

- First paragraph item.

  Second paragraph in the same item.

- Another item.

```go
func main() {
	fmt.Println("fenced code, no highlighting")
}
```

Setext heading level 2
----------------------

    indented code block
    second line

| Left | Centre | Right | None |
| :--- | :----: | ----: | ---- |
| a    |   b    |     c | d    |
| `e`  | **f**  |     g | h    |
| i    |   j    |     k | l    |

---

![A red rectangle](img/red-4x3.png)

![A blue square](img/blue-8x8.jpg)

![A missing image](img/missing.png)

![A remote image](https://example.com/remote.png)

[![A linked red rectangle](img/red-4x3.png)](https://example.com/)

A [reference link][ref].

[ref]: https://example.com/ref

<div align="center">HTML is not rendered.</div>

Text with <span>inline HTML</span> around it.

<!-- an HTML comment -->

The end.
