package generation

const koreanBlockFieldContract = "TEXT와 QUOTE의 본문은 content에, HEADING의 제목은 content와 level에 쓰며 level은 2 또는 3입니다. LIST의 각 항목은 items 배열의 문자열이며 LIST 본문을 content에 넣지 않습니다. IMAGE는 file, alt, caption을 쓰고 file에는 첨부 사진 파일명만 넣습니다. GALLERY는 files, layout, alt, caption을 쓰며 file은 비웁니다. 해당 블록에 쓰지 않는 필드는 비웁니다."
const englishBlockFieldContract = "TEXT and QUOTE prose goes in content; HEADING uses content and level, with level 2 or 3. LIST holds each item as a string in items, never as prose in content. IMAGE uses file, alt, and caption, with an attached photo filename in file. GALLERY uses files, layout, alt, and caption and leaves file empty. Leave fields unrelated to the block empty."
