package generation

import (
	"slices"
	"sort"

	"github.com/postpilot/backend/internal/post"
)

type originRevisionFieldPair struct {
	from, to post.OriginFieldLocator
	regions  []originUnchangedRange
}

func normalizedOriginBlockKey(block Block) string {
	if len(block.Items) == 0 {
		block.Items = nil
	}
	if len(block.Files) == 0 {
		block.Files = nil
	}
	return originHash(block)
}

// provenOriginSequencePairs recognizes unique exact retained values first.
// Changed values pair only in a single remaining compatible slot, or a single
// slot between monotonic exact anchors. It never pairs ambiguous duplicate runs.
func provenOriginSequencePairs(before, after []string, compatible func(int, int) bool) (map[int]int, map[int]bool) {
	old, next := map[string][]int{}, map[string][]int{}
	for i, key := range before {
		old[key] = append(old[key], i)
	}
	for i, key := range after {
		next[key] = append(next[key], i)
	}
	pairs, occupied := map[int]int{}, map[int]bool{}
	for key, indices := range old {
		if len(indices) == 1 && len(next[key]) == 1 {
			pairs[indices[0]] = next[key][0]
			occupied[next[key][0]] = true
		}
	}
	var oldRemaining, nextRemaining []int
	for i := range before {
		if _, known := pairs[i]; !known {
			oldRemaining = append(oldRemaining, i)
		}
	}
	for i := range after {
		if !occupied[i] {
			nextRemaining = append(nextRemaining, i)
		}
	}
	if len(oldRemaining) == 1 && len(nextRemaining) == 1 && compatible(oldRemaining[0], nextRemaining[0]) {
		pairs[oldRemaining[0]], occupied[nextRemaining[0]] = nextRemaining[0], true
	}
	var anchors []int
	for index := range pairs {
		anchors = append(anchors, index)
	}
	sort.Ints(anchors)
	monotonic, previous := true, -1
	for _, index := range anchors {
		if pairs[index] <= previous {
			monotonic = false
		}
		previous = pairs[index]
	}
	if monotonic {
		oldStart, newStart := -1, -1
		for _, oldEnd := range append(anchors, len(before)) {
			newEnd := len(after)
			if oldEnd < len(before) {
				newEnd = pairs[oldEnd]
			}
			if oldEnd-oldStart == 2 && newEnd-newStart == 2 && compatible(oldStart+1, newStart+1) {
				pairs[oldStart+1], occupied[newStart+1] = newStart+1, true
			}
			oldStart, newStart = oldEnd, newEnd
		}
	}
	unpairedOld := false
	for i := range before {
		if _, exists := pairs[i]; !exists {
			unpairedOld = true
		}
	}
	ambiguous := map[int]bool{}
	if unpairedOld {
		for i := range after {
			if !occupied[i] {
				ambiguous[i] = true
			}
		}
	}
	return pairs, ambiguous
}

func revisionOriginFieldPairs(before, after PostContent) (map[originFieldKey]originRevisionFieldPair, map[originFieldKey]bool) {
	pairs, ambiguous := map[originFieldKey]originRevisionFieldPair{}, map[originFieldKey]bool{}
	beforeContent, afterContent := originPostContent(before), originPostContent(after)
	add := func(from, to post.OriginFieldLocator) {
		old, valid := post.OriginFieldText(beforeContent, from)
		next, exists := post.OriginFieldText(afterContent, to)
		if valid && exists {
			pairs[originKey(from)] = originRevisionFieldPair{from: cloneOriginLocator(from), to: cloneOriginLocator(to), regions: unchangedOriginRanges(old, next)}
		}
	}
	for _, kind := range []post.OriginFieldKind{post.OriginFieldTitle, post.OriginFieldSummary} {
		field := post.OriginFieldLocator{Kind: kind}
		add(field, field)
	}
	tags, ambiguousTags := provenOriginSequencePairs(before.Tags, after.Tags, func(int, int) bool { return true })
	for old, next := range tags {
		add(post.OriginFieldLocator{Kind: post.OriginFieldTag, TagIndex: cloneOriginIndex(&old)}, post.OriginFieldLocator{Kind: post.OriginFieldTag, TagIndex: cloneOriginIndex(&next)})
	}
	for index := range ambiguousTags {
		ambiguous[originKey(post.OriginFieldLocator{Kind: post.OriginFieldTag, TagIndex: &index})] = true
	}
	oldKeys, newKeys := make([]string, len(before.Blocks)), make([]string, len(after.Blocks))
	for i, block := range before.Blocks {
		oldKeys[i] = normalizedOriginBlockKey(block)
	}
	for i, block := range after.Blocks {
		newKeys[i] = normalizedOriginBlockKey(block)
	}
	blocks, ambiguousBlocks := provenOriginSequencePairs(oldKeys, newKeys, func(old, next int) bool {
		left, right := before.Blocks[old], after.Blocks[next]
		return left.Type == right.Type && left.File == right.File && slices.Equal(left.Files, right.Files)
	})
	for old, next := range blocks {
		left, right := before.Blocks[old], after.Blocks[next]
		from := func(kind post.OriginFieldKind) post.OriginFieldLocator {
			return post.OriginFieldLocator{Kind: kind, BlockIndex: cloneOriginIndex(&old)}
		}
		to := func(kind post.OriginFieldKind) post.OriginFieldLocator {
			return post.OriginFieldLocator{Kind: kind, BlockIndex: cloneOriginIndex(&next)}
		}
		switch left.Type {
		case BlockText, BlockHeading, BlockQuote:
			add(from(post.OriginFieldBlockContent), to(post.OriginFieldBlockContent))
		case BlockImage, BlockVideo, BlockGallery:
			add(from(post.OriginFieldBlockAlt), to(post.OriginFieldBlockAlt))
			add(from(post.OriginFieldBlockCaption), to(post.OriginFieldBlockCaption))
		case BlockList:
			items, ambiguousItems := provenOriginSequencePairs(left.Items, right.Items, func(int, int) bool { return true })
			for oldItem, nextItem := range items {
				first, second := from(post.OriginFieldBlockItem), to(post.OriginFieldBlockItem)
				first.ItemIndex, second.ItemIndex = cloneOriginIndex(&oldItem), cloneOriginIndex(&nextItem)
				add(first, second)
			}
			for item := range ambiguousItems {
				field := to(post.OriginFieldBlockItem)
				field.ItemIndex = &item
				ambiguous[originKey(field)] = true
			}
		}
	}
	for _, field := range readableOriginFields(after) {
		if field.BlockIndex != nil && ambiguousBlocks[*field.BlockIndex] {
			ambiguous[originKey(field)] = true
		}
	}
	return pairs, ambiguous
}
