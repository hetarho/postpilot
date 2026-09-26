import { describe, expect, it } from 'vitest'
import fixture from './region-layouts.fixture.json'
import slotFixture from './region-slots.fixture.json'
import design from '../config/clip-design.json'
import {
  clipFitRegionSlot,
  clipLayoutRegion,
  clipRegionSlotAt,
  clipRegionSlotBudget,
  clipRegionSlots,
  type ClipRegionKind,
  type ClipRegionRatio,
} from './region-layout'

const close = (a: number, b: number) => Math.abs(a - b) < 0.01

describe('the region layout port', () => {
  it('reproduces the Go layout of every preset on every ratio (CDS-86, CDS-87)', () => {
    expect(fixture.length).toBe(135)
    for (const c of fixture) {
      const got = clipLayoutRegion(
        c.kind as ClipRegionKind,
        c.id,
        c.ratio as ClipRegionRatio,
        c.rows,
      )
      const name = `${c.kind}.${c.id} ${c.ratio} ${JSON.stringify(c.rows)}`
      expect(got, name).toBeDefined()
      expect(close(got!.anchorX, c.anchorX), name).toBe(true)
      expect(got!.scrim, name).toBe(c.scrim)
      expect(got!.slots.length, name).toBe(c.slots.length)
      c.slots.forEach((want, i) => {
        const slot = got!.slots[i]
        expect([slot.index, slot.type.floor, slot.over], name).toEqual([
          want.index,
          want.floor,
          want.over,
        ])
        expect(slot.lines.length, name).toBe(want.lines.length)
        want.lines.forEach((line, k) => {
          const drawn = slot.lines[k]
          expect(drawn.text, name).toBe(line.text)
          expect(drawn.size, name).toBe(line.size)
          expect(close(drawn.baseline, line.baseline), `${name} baseline`).toBe(true)
          expect(close(drawn.width, line.width), `${name} width`).toBe(true)
          expect(close(drawn.x, line.x), `${name} x`).toBe(true)
          expect([drawn.align, drawn.rotated], name).toEqual([
            line.align,
            (line as { rotated?: boolean }).rotated ?? false,
          ])
          const arc = (line as { arc?: { cx: number; cy: number; r: number; lower: boolean } }).arc
          expect(!!drawn.arc, `${name} arc`).toBe(!!arc)
          if (arc && drawn.arc) {
            expect(close(drawn.arc.cx, arc.cx) && close(drawn.arc.cy, arc.cy), name).toBe(true)
            expect([drawn.arc.r, drawn.arc.lower], name).toEqual([arc.r, arc.lower])
          }
        })
      })
      expect(got!.rules.length, name).toBe(c.rules.length)
      c.rules.forEach((want, i) => {
        const rule = got!.rules[i]
        expect(rule.kind).toBe(want.kind)
        expect(close(rule.box.x, want.x) && close(rule.box.y, want.y), `${name} rule`).toBe(true)
        expect([rule.box.width, rule.box.height]).toEqual([want.w, want.h])
      })
      // Every piece of neutral decoration at the renderer's place (CDS-88).
      expect(got!.shapes.length, `${name} shapes`).toBe(c.shapes.length)
      c.shapes.forEach((raw, i) => {
        const want = raw as (typeof c.shapes)[number] & {
          circle?: boolean
          fill?: string
          fillAlpha?: number
          stroke?: string
          strokeAlpha?: number
          strokeWidth?: number
          shadow?: boolean
          rotated?: boolean
        }
        const shape = got!.shapes[i]
        const at = `${name} ${want.kind} ${i}`
        expect([shape.kind, shape.slot], at).toEqual([want.kind, want.slot])
        const b = shape.box
        expect(
          close(b.x, want.x) &&
            close(b.y, want.y) &&
            close(b.width, want.w) &&
            close(b.height, want.h),
          `${at} box ${JSON.stringify(b)}`,
        ).toBe(true)
        expect(close(shape.radius, want.radius), at).toBe(true)
        expect(
          [
            shape.circle,
            shape.fill,
            shape.fillAlpha,
            shape.stroke,
            shape.strokeAlpha,
            shape.strokeWidth,
            shape.shadow,
            shape.rotated,
          ],
          at,
        ).toEqual([
          want.circle ?? false,
          want.fill ?? '',
          want.fillAlpha ?? 0,
          want.stroke ?? '',
          want.strokeAlpha ?? 0,
          want.strokeWidth ?? 0,
          want.shadow ?? false,
          want.rotated ?? false,
        ])
      })
      const rotate = (c as { rotate?: { deg: number; cx: number; cy: number } }).rotate
      expect(got!.rotate.deg, `${name} rotate`).toBe(rotate?.deg ?? 0)
      if (rotate) {
        expect(close(got!.rotate.cx, rotate.cx) && close(got!.rotate.cy, rotate.cy), name).toBe(
          true,
        )
      }
    }
  })

  it('shrinks, then wraps, then overflows past the floor', () => {
    const headline = {
      role: 'headline',
      size: 96,
      floor: 60,
      face: 'paperlogy',
      weight: 800,
      tracking: -0.02,
      lines: 0,
    }
    expect(clipFitRegionSlot(headline, '해미 한우', 856)).toEqual({
      lines: ['해미 한우'],
      size: 96,
      over: false,
    })
    expect(
      clipFitRegionSlot(headline, '연남동 골목에서 30년째 숯불 한우만 굽는 집', 856).lines,
    ).toHaveLength(2)
    expect(clipFitRegionSlot(headline, '하나둘셋넷'.repeat(5), 856).over).toBe(true)
    expect(clipRegionSlotBudget(headline, 856)).toBeGreaterThanOrEqual(30)
  })

  it('gives every preset slot the Go fit spec and width, group slots included (CLIP-116)', () => {
    expect(slotFixture.length).toBeGreaterThan(100)
    for (const c of slotFixture) {
      const at = clipRegionSlotAt(
        c.kind as ClipRegionKind,
        c.id,
        c.ratio as ClipRegionRatio,
        c.index,
      )
      const name = `${c.kind}.${c.id} ${c.ratio} slot ${c.index}`
      expect(at, name).toBeDefined()
      expect([at!.spec.role, at!.spec.size, at!.spec.floor], name).toEqual([
        c.role,
        c.size,
        c.floor,
      ])
      expect(clipRegionSlotBudget(at!.spec, at!.width), `${name} budget`).toBe(c.budget)
      expect(close(at!.width, c.width), `${name} width`).toBe(true)
    }
    expect(clipRegionSlots('outro', 'chips')).toHaveLength(6)
  })

  it('lays out every preset in design.json, none left to the renderer alone', () => {
    for (const kind of ['intro', 'outro'] as const) {
      for (const id of Object.keys(design.regions[kind])) {
        const rows = clipRegionSlots(kind, id).map((_, i) => `슬롯 ${i + 1}`)
        const got = clipLayoutRegion(kind, id, 'vertical', rows)
        expect(got?.slots.length, `${kind}.${id}`).toBe(rows.length)
      }
    }
  })
})
