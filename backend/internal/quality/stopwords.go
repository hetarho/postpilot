package quality

import "strings"

// The words a phrase made only of says nothing (QUAL-26): conjunctions, particles standing
// alone, intensifiers, demonstratives and the most common verb forms. Product data rather than
// grammar — changed here, with no schema change — and a starter list, not a complete one.
var koreanStopwords = setOf(`및 등 그리고 그러나 그런데 하지만 그래서 또는 또한 또 더 덜 잘 좀 꼭 너무 정말 진짜
아주 매우 많이 조금 가장 제일 그 이 저 그런 이런 저런 어떤 모든 각 것 거 수 때 중 위해 통해 대한 대해 있는
있다 있어요 없는 없다 하는 한 할 합니다 했다 해요 하고 된 되는 입니다 같은 나 저는 제가 우리 저희`)

var englishStopwords = setOf(`a an the and or but of to in on at by for with from as is are was were be been it its
this that these those i you we they my your our me us`)

func setOf(words string) map[string]bool {
	out := map[string]bool{}
	for _, word := range strings.Fields(words) {
		out[word] = true
	}
	return out
}

// isStopword matches a Korean token exactly and an English one after lowering its case.
func isStopword(token string) bool {
	return koreanStopwords[token] || englishStopwords[strings.ToLower(token)]
}
