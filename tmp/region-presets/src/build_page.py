# -*- coding: utf-8 -*-
"""out/web 이미지 + fit.json + 프리셋 목록 → out/page.html, out/files.json(게시할 파일 목록)."""
import json, os
import presets
from lib import OUT

HERE = os.path.dirname(os.path.abspath(__file__))

DESC = {
    "intro": {
        "A": "지금 기본값.",
        "B": "가는 선 두 줄 사이에 큰 줄.",
        "i01": "작은 줄 위, 큰 줄 아래, 짧은 흰 바로 마무리.",
        "i02": "화면 위쪽에 왼쪽 정렬로 크게. 피사체가 가운데일 때.",
        "i03": "명조 큰 줄, 첫 줄 양옆에 짧은 선.",
        "i04": "큰 줄을 얇은 사각 테두리로 감싼다. 테두리가 글자 폭을 따라간다.",
        "i05": "첫 슬롯을 속 빈 외곽선으로 아주 크게.",
        "i06": "아래 왼쪽, 점 · 큰 줄 · 가는 선 · 작은 줄.",
        "i07": "뷰파인더처럼 네 귀퉁이만 그린다.",
        "i08": "큰 줄이 줄마다 화면 폭을 가득 채운다.",
        "i09": "위에 큰 줄, 아래에 작은 줄. 가운데는 피사체에 비워 둔다.",
        "i10": "두꺼운 외곽선 주아 글자를 살짝 기울이고 아래 어두운 칩.",
    },
    "outro": {
        "B": "큰 줄 + 가는 선 + 본문.",
        "E": "작은 줄 · 아주 큰 줄 · 바 · 캡션.",
        "o01": "둘째 줄을 알약 모양 테두리로 감싼다.",
        "o02": "어두운 판 위에 머리글과 네 줄 목록.",
        "o03": "가운데 세로 선을 두고 왼쪽 큰 줄, 오른쪽 작은 두 줄.",
        "o04": "명조 큰 줄 + 짧은 선 + 작은 두 줄.",
        "o05": "화면 가로 전체에 어두운 띠를 깔고 그 안에 두 줄.",
        "o06": "아래 왼쪽, 흰 세로 막대 옆에 세 줄.",
        "o07": "큰 줄 + 알약 칩 최대 다섯 개, 넘치면 다음 줄.",
        "o08": "가는 선 두 줄 사이에 화면 폭을 채우는 아주 큰 한 줄.",
        "o09": "큰 줄 + 작은 네모 표시가 붙은 세 줄.",
        "o10": "두 겹 원 띠를 따라 두 줄이 돌고 가운데 큰 줄.",
    },
}
TAGS = {
    "intro": {"A": [["now", "현재"]], "B": [["now", "현재"]], "i02": [["", "위쪽 스크림"]], "i03": [["spec", "나눔명조"]],
              "i06": [["", "아래쪽 스크림"]], "i09": [["", "위아래 스크림"]], "i10": [["spec", "주아"], ["spec", "판"]]},
    "outro": {"B": [["now", "현재"]], "E": [["now", "현재"]], "o02": [["spec", "판"]], "o04": [["spec", "나눔명조"]],
              "o05": [["spec", "띠"]], "o06": [["", "아래쪽 스크림"]]},
}


def meta(kind, items):
    return [dict(id=p["id"], title=p["title"], slots=p["slots"], where=p["where"], desc=DESC[kind][p["id"]],
                 tags=TAGS[kind].get(p["id"], [])) for p in items]


data = dict(fit=json.load(open(os.path.join(OUT, "fit.json"))), intro=meta("intro", presets.INTROS), outro=meta("outro", presets.OUTROS))
tpl = open(os.path.join(HERE, "page.tpl.html")).read()
with open(os.path.join(OUT, "page.html"), "w") as fh:
    fh.write(tpl.replace("__DATA__", json.dumps(data, ensure_ascii=False)))

files = {}
for name in sorted(os.listdir(os.path.join(OUT, "web", "full"))):
    files["img/full/" + name] = "web/full/" + name
    files["img/thumb/" + name] = "web/thumb/" + name
json.dump(files, open(os.path.join(OUT, "files.json"), "w"), ensure_ascii=False)
print(len(files), "files")
