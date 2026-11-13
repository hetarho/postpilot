# REVIEW design-system-panels-261114
> st:converted@261114 | scope:frontend/src | at:00bf4209b | base:ARCH@24

## summary
The exhaustive visual audit found remaining generic bordered panels and independently styled resource surfaces outside the shared design language.

## findings
- F1 [o] P2 frontend/src/features/create-template-wizard/ui/ContentKindCard.tsx and frontend/src/entities/template/ui/PreviewStandIns.tsx: generic example frames, choice cards and nested sample panels use resting borders instead of shared surfaces ← the template wizard still presents a different visual language. →T821
- F2 [o] P2 frontend/src/pages/admin/ui and frontend/src/widgets/export-panel/ui/ExportPanel.tsx: administrative resource cards and export fallback panels compose their own resting planes; writing-test history draws per-item borders instead of the shared row separator ← secondary routes drift from the product-wide card and list contract. →T822

- F3 [o] P2 catalog/recommendation/model-pair/voucher feature panels still draw independent legacy raised boxes; the route-level migration did not cover embedded features. →T823
- F4 [o] P2 inline block editing, generated voice candidates and observation carousels retain old generic surfaces; harmonize independent cards and inner wells without changing content/media. →T824
- F5 [o] P2 desktop comparison panels and plan-estimator conditions retain independent old card recipes; preserve deliberate plain phone comparisons and financial calculations. →T825

- F6 [o] P1 frontend/src/pages/editor/ui/EditorPage.tsx: failure reads retain a legacy text return and pending placeholders omit the focused workflow return; final matrix found it in both themes. Preserve one shared named return throughout recovery. →T826

## notes
- Adopted scope is the explicit gusta-start request to migrate every nonconforming product screen, under ARCH-20 and THEME-12/13/78. Changes preserve every action, value, state and chronological/injection order; promotional frames, data-table rules, selection rings and content drawings retain their named exceptions.
- Refund/billing controls are already assigned to T815. Native buttons and fields remain in T810/T813/T814/T815; this review adds only uncovered panel/example surfaces.
