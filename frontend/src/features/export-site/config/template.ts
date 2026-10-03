/** Fixed, content-independent design. Keeping this constant makes every exported page consistent. */
export const SITE_STYLE = `
:root { font-family: system-ui, sans-serif; line-height: 1.7; }
body { max-width: 44rem; margin: 0 auto; padding: 2rem 1rem 4rem; }
h1, h2, h3 { line-height: 1.25; }
.summary { font-size: 1.1rem; }
.meta, .tags { font-size: 0.875rem; }
.tags { display: flex; flex-wrap: wrap; gap: 0.5rem; padding: 0; list-style: none; }
figure { margin: 2rem 0; }
img { display: block; width: 100%; height: auto; }
figcaption { margin-top: 0.5rem; font-size: 0.875rem; }
.gallery .photos { gap: 0.5rem; }
.gallery-collage .photos { display: grid; grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr)); }
.gallery-collage img { aspect-ratio: 1; object-fit: cover; }
.gallery-slide .photos { display: flex; overflow-x: auto; scroll-snap-type: x mandatory; }
.gallery-slide img { flex: 0 0 100%; scroll-snap-align: center; }
blockquote { margin: 1.5rem 0; padding-left: 1rem; }
`.trim()

export const SITE_DOCUMENT_PREFIX =
  '<!doctype html>\n<html lang="{{language}}">\n<head>\n<meta charset="utf-8">\n<meta name="viewport" content="width=device-width, initial-scale=1">'
export const SITE_DOCUMENT_SUFFIX = '</body>\n</html>'
