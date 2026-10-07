import { Code, createRouterTransport } from '@connectrpc/connect'
import { create, type MessageInitShape } from '@bufbuild/protobuf'
import {
  AddVoiceSampleResponseSchema,
  UpdateVoiceSampleResponseSchema,
  CreateVoiceResponseSchema,
  DeleteVoiceResponseSchema,
  DeleteVoiceSampleResponseSchema,
  GetVoiceProfileResponseSchema,
  ListVoicesResponseSchema,
  RenameVoiceResponseSchema,
  RestoreVoiceResponseSchema,
  SetDefaultVoiceResponseSchema,
  VoiceProfileSchema,
  VoiceSampleSchema,
  VoiceSchema,
  VoiceService,
  VoiceAnalysisSchema,
  VoiceNoticeKind,
  VoiceNoticeSchema,
  RestorePreviousVoiceAnalysisResponseSchema,
  GetPostFingerprintResponseSchema,
  VoiceCheckSchema,
  ProtoVoiceCheckStatus,
  ListVoiceChecksResponseSchema,
  StartVoiceCheckResponseSchema,
  RetryVoiceCheckResponseSchema,
  type AppFailureReason,
  type ProtoVoiceCheck,
  type ProtoVoiceProfile,
  type ProtoVoiceSample,
  type ProtoVoiceAnalysis,
  VoicePromptPart,
  VoiceSampleKind,
  VoiceReadinessSchema,
  VoicePromptSchema,
  ListVoicePromptsResponseSchema,
  GetVoiceSampleResponseSchema,
  CreateVoicePhotoUploadResponseSchema,
  AnswerVoicePromptResponseSchema,
  AnalyzeVoiceResponseSchema,
} from '@/shared/api'
import { connectAppError } from './app-error'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]

export interface FakeVoiceSampleRow {
  id: string
  label: string
  chars?: number
  createdAt?: string
  kind?: 'post' | 'answer'
  promptKey?: string
  /** The full text; omitted, a 200-character body. */
  body?: string
  hasPhoto?: boolean
  contentRevision?: bigint
}

/** The shared prompt set as the fake serves it: a subset with every part and a photo prompt,
 *  which is all a screen test needs to tell groups and photos apart. */
export const FAKE_VOICE_PROMPTS = [
  {
    key: 'starter_day_open',
    part: 'opening',
    photo: false,
    starter: true,
    text: '어떤 인사로 이야기를 시작할까요? 평소 말투로 1~3문장이면 충분해요.',
    scene: '여유로운 주말 아침이에요. 친구에게 오늘 계획을 이야기하려고 해요.',
    hint: '거창한 계획이 아니어도 좋아요. 집에서 쉬는 이야기도 괜찮아요.',
  },
  {
    key: 'starter_discovery_open',
    part: 'opening',
    photo: false,
    starter: true,
    text: '친구에게 이 가게를 소개하는 첫마디를 써 보세요. 평소 말투로 1~3문장이면 충분해요.',
    scene: '산책하다가 작고 조용한 가게를 발견했어요.',
    hint: '실제 가게가 아니어도 괜찮아요. 처음 눈길이 간 점부터 말해 주세요.',
  },
  {
    key: 'starter_snack',
    part: 'description',
    photo: false,
    starter: true,
    text: '친구에게 그 맛을 어떻게 설명할까요? 평소 말투로 1~3문장이면 충분해요.',
    scene: '별 기대 없이 고른 간식이 한입 먹어 보니 생각보다 맛있어요.',
    hint: '맛있다는 말에 이유 하나를 더해 보세요.',
  },
  {
    key: 'starter_rain',
    part: 'description',
    photo: false,
    starter: true,
    text: '그 순간의 마음을 친구에게 이야기해 보세요. 평소 말투로 1~3문장이면 충분해요.',
    scene: '나가려는데 갑자기 비가 내려요. 다행히 우산은 챙겼어요.',
    hint: '반가웠는지, 귀찮았는지 편하게 써 주세요.',
  },
  {
    key: 'starter_recommend',
    part: 'description',
    photo: false,
    starter: true,
    text: '어떤 일을 추천하고 싶나요? 평소 말투로 1~3문장이면 충분해요.',
    scene: '친구가 주말에 부담 없이 기분 전환을 하고 싶대요.',
    hint: '산책이나 낮잠처럼 작은 일도 좋아요.',
  },
  {
    key: 'starter_inconvenience',
    part: 'description',
    photo: false,
    starter: true,
    text: '아쉬운 점을 어떻게 말할까요? 평소 말투로 1~3문장이면 충분해요.',
    scene: '마음에 들어서 산 컵이 막상 써 보니 손잡이가 조금 불편해요.',
    hint: '좋았던 점과 불편한 점을 같이 이야기해도 좋아요.',
  },
  {
    key: 'starter_surprise',
    part: 'description',
    photo: false,
    starter: true,
    text: '뜻밖에 편해진 순간을 써 보세요. 평소 말투로 1~3문장이면 충분해요.',
    scene: '평소 붐비던 길이 오늘은 한산해서 걸어가기 편해요.',
    hint: '놀랐던 마음이나 달라진 기분을 써 주세요.',
  },
  {
    key: 'starter_choice',
    part: 'description',
    photo: false,
    starter: true,
    text: '어느 쪽을 고르고 싶은지 이유를 써 보세요. 평소 말투로 1~3문장이면 충분해요.',
    scene: '오늘은 새로운 메뉴를 먹을지, 늘 먹던 메뉴를 고를지 고민돼요.',
    hint: '정답은 없어요. 지금 마음이 가는 쪽이면 돼요.',
  },
  {
    key: 'starter_day_close',
    part: 'closing',
    photo: false,
    starter: true,
    text: '오늘 이야기를 마지막 한마디로 마무리해 보세요. 평소 말투로 1~3문장이면 충분해요.',
    scene: '작은 산책을 마치고 집으로 돌아왔어요.',
    hint: '오늘 기분이나 다음에 하고 싶은 일을 말해도 좋아요.',
  },
  {
    key: 'starter_friend_close',
    part: 'closing',
    photo: false,
    starter: true,
    text: '친구에게 건넬 끝인사를 써 보세요. 평소 말투로 1~3문장이면 충분해요.',
    scene: '친구에게 즐거웠던 하루를 다 이야기했어요.',
    hint: '평소 메시지에서 쓰는 편한 인사면 충분해요.',
  },
  {
    key: 'opening_greeting',
    part: 'opening',
    photo: false,
    starter: false,
    text: '첫인사를 하고 이야기를 시작해 보세요. 평소 말투로 1~3문장이면 충분해요.',
    scene: '친구에게 오늘 하루 이야기를 들려주려고 해요.',
    hint: '실제 경험이 없어도 이 상황을 가볍게 상상해 보세요.',
  },
  {
    key: 'opening_topic',
    part: 'opening',
    photo: false,
    starter: false,
    text: '어떤 물건인지 먼저 꺼내 보세요. 평소 말투로 1~3문장이면 충분해요.',
    scene: '요즘 마음에 드는 작은 물건을 하나 소개하려고 해요.',
    hint: '실제 경험이 없어도 이 상황을 가볍게 상상해 보세요.',
  },
  {
    key: 'opening_reason',
    part: 'opening',
    photo: false,
    starter: false,
    text: '왜 눈길이 갔는지 이야기의 첫 부분을 써 보세요. 평소 말투로 1~3문장이면 충분해요.',
    scene: '산책하다가 처음 보는 가게에 잠깐 들어가기로 했어요.',
    hint: '실제 경험이 없어도 이 상황을 가볍게 상상해 보세요.',
  },
  {
    key: 'opening_return',
    part: 'opening',
    photo: false,
    starter: false,
    text: '반가운 인사로 이야기를 시작해 보세요. 평소 말투로 1~3문장이면 충분해요.',
    scene: '한동안 연락하지 못한 친구에게 근황을 전하려고 해요.',
    hint: '실제 경험이 없어도 이 상황을 가볍게 상상해 보세요.',
  },
  {
    key: 'photo_food',
    part: 'description',
    photo: true,
    starter: false,
    text: '사진 속 맛과 느낌을 친구에게 이야기해 보세요. 1~3문장이면 충분해요.',
    scene: '직접 찍은 음식이나 음료 사진 한 장을 골라 주세요.',
    hint: '직접 찍은 사진을 골라, 친구에게 보여주듯 써 주세요.',
  },
  {
    key: 'photo_space',
    part: 'description',
    photo: true,
    starter: false,
    text: '사진에서 보이는 분위기를 소개해 보세요. 1~3문장이면 충분해요.',
    scene: '직접 찍은 가게나 공간 사진 한 장을 골라 주세요.',
    hint: '직접 찍은 사진을 골라, 친구에게 보여주듯 써 주세요.',
  },
  {
    key: 'photo_item',
    part: 'description',
    photo: true,
    starter: false,
    text: '이 물건을 써 본 느낌을 이야기해 보세요. 1~3문장이면 충분해요.',
    scene: '직접 찍은 물건 사진 한 장을 골라 주세요.',
    hint: '직접 찍은 사진을 골라, 친구에게 보여주듯 써 주세요.',
  },
  {
    key: 'photo_scenery',
    part: 'description',
    photo: true,
    starter: false,
    text: '이 풍경을 보고 든 마음을 이야기해 보세요. 1~3문장이면 충분해요.',
    scene: '직접 찍은 풍경 사진 한 장을 골라 주세요.',
    hint: '직접 찍은 사진을 골라, 친구에게 보여주듯 써 주세요.',
  },
  {
    key: 'photo_info',
    part: 'description',
    photo: true,
    starter: false,
    text: '눈에 띄는 정보를 친구에게 쉽게 설명해 보세요. 1~3문장이면 충분해요.',
    scene: '직접 찍은 메뉴나 안내판 사진 한 장을 골라 주세요.',
    hint: '직접 찍은 사진을 골라, 친구에게 보여주듯 써 주세요.',
  },
  {
    key: 'photo_favorite',
    part: 'description',
    photo: true,
    starter: false,
    text: '이 사진이 좋은 이유를 이야기해 보세요. 1~3문장이면 충분해요.',
    scene: '직접 찍은 사진 중 마음에 드는 한 장을 골라 주세요.',
    hint: '직접 찍은 사진을 골라, 친구에게 보여주듯 써 주세요.',
  },
  {
    key: 'situation_first_visit',
    part: 'description',
    photo: false,
    starter: false,
    text: '문을 열었을 때 느낀 분위기를 써 보세요. 평소 말투로 1~3문장이면 충분해요.',
    scene: '처음 들어간 가게에 은은한 음악이 흐르고 있어요.',
    hint: '실제 경험이 없어도 이 상황을 가볍게 상상해 보세요.',
  },
  {
    key: 'situation_better',
    part: 'description',
    photo: false,
    starter: false,
    text: '한입 먹고 든 생각을 전해 보세요. 평소 말투로 1~3문장이면 충분해요.',
    scene: '별 기대 없이 고른 간식이 생각보다 맛있어요.',
    hint: '실제 경험이 없어도 이 상황을 가볍게 상상해 보세요.',
  },
  {
    key: 'situation_worse',
    part: 'description',
    photo: false,
    starter: false,
    text: '아쉬운 점을 솔직하게 전해 보세요. 평소 말투로 1~3문장이면 충분해요.',
    scene: '큰 기대를 하고 산 물건이 막상 써 보니 조금 불편해요.',
    hint: '실제 경험이 없어도 이 상황을 가볍게 상상해 보세요.',
  },
  {
    key: 'situation_recommend',
    part: 'description',
    photo: false,
    starter: false,
    text: '부담 없이 해 볼 만한 일을 하나 추천해 보세요. 평소 말투로 1~3문장이면 충분해요.',
    scene: '친구가 주말에 뭘 할지 고민하고 있어요.',
    hint: '실제 경험이 없어도 이 상황을 가볍게 상상해 보세요.',
  },
  {
    key: 'situation_waiting',
    part: 'description',
    photo: false,
    starter: false,
    text: '그때의 마음을 친구에게 이야기해 보세요. 평소 말투로 1~3문장이면 충분해요.',
    scene: '약속 장소를 찾다가 같은 골목을 두 번 돌았어요.',
    hint: '실제 경험이 없어도 이 상황을 가볍게 상상해 보세요.',
  },
  {
    key: 'situation_value',
    part: 'description',
    photo: false,
    starter: false,
    text: '어느 쪽이 마음에 드는지 이유를 써 보세요. 평소 말투로 1~3문장이면 충분해요.',
    scene: '양이 넉넉한 간식과 조금 비싼 작은 간식 중 고르려 해요.',
    hint: '실제 경험이 없어도 이 상황을 가볍게 상상해 보세요.',
  },
  {
    key: 'closing_greeting',
    part: 'closing',
    photo: false,
    starter: false,
    text: '가벼운 끝인사로 마무리해 보세요. 평소 말투로 1~3문장이면 충분해요.',
    scene: '친구에게 오늘 이야기를 다 들려줬어요.',
    hint: '실제 경험이 없어도 이 상황을 가볍게 상상해 보세요.',
  },
  {
    key: 'closing_return',
    part: 'closing',
    photo: false,
    starter: false,
    text: '다시 들르고 싶은지 마지막 한마디를 써 보세요. 평소 말투로 1~3문장이면 충분해요.',
    scene: '우연히 들른 가게가 꽤 마음에 들었어요.',
    hint: '실제 경험이 없어도 이 상황을 가볍게 상상해 보세요.',
  },
  {
    key: 'closing_reader',
    part: 'closing',
    photo: false,
    starter: false,
    text: '친구에게 건네고 싶은 마지막 말을 써 보세요. 평소 말투로 1~3문장이면 충분해요.',
    scene: '친구가 바쁜 와중에도 이야기를 끝까지 들어줬어요.',
    hint: '실제 경험이 없어도 이 상황을 가볍게 상상해 보세요.',
  },
  {
    key: 'closing_summary',
    part: 'closing',
    photo: false,
    starter: false,
    text: '오늘 느낀 점으로 끝맺어 보세요. 평소 말투로 1~3문장이면 충분해요.',
    scene: '짧은 산책 이야기를 마무리하려고 해요.',
    hint: '실제 경험이 없어도 이 상황을 가볍게 상상해 보세요.',
  },
  {
    key: 'everyday_food_01',
    part: 'description',
    photo: false,
    starter: false,
    text: '한입 먹었을 때의 느낌을 친구에게 전해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '따뜻한 국 한 숟갈을 먹으니 몸이 조금 풀려요.',
    hint: '맛이나 식감 중 떠오르는 것 하나만 이야기해도 좋아요.',
  },
  {
    key: 'everyday_food_02',
    part: 'description',
    photo: false,
    starter: false,
    text: '한입 먹었을 때의 느낌을 친구에게 전해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '갓 구운 빵을 나눠 먹는데 가장자리가 바삭해요.',
    hint: '맛이나 식감 중 떠오르는 것 하나만 이야기해도 좋아요.',
  },
  {
    key: 'everyday_food_03',
    part: 'description',
    photo: false,
    starter: false,
    text: '한입 먹었을 때의 느낌을 친구에게 전해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '같은 간식을 먹어도 오늘은 유난히 달게 느껴져요.',
    hint: '맛이나 식감 중 떠오르는 것 하나만 이야기해도 좋아요.',
  },
  {
    key: 'everyday_food_04',
    part: 'description',
    photo: false,
    starter: false,
    text: '한입 먹었을 때의 느낌을 친구에게 전해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '매운 음식을 먹었는데 예상보다 훨씬 매워요.',
    hint: '맛이나 식감 중 떠오르는 것 하나만 이야기해도 좋아요.',
  },
  {
    key: 'everyday_food_05',
    part: 'description',
    photo: false,
    starter: false,
    text: '한입 먹었을 때의 느낌을 친구에게 전해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '냉장고에 남은 재료로 간단한 한 끼를 만들었어요.',
    hint: '맛이나 식감 중 떠오르는 것 하나만 이야기해도 좋아요.',
  },
  {
    key: 'everyday_food_06',
    part: 'description',
    photo: false,
    starter: false,
    text: '한입 먹었을 때의 느낌을 친구에게 전해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '점심으로 먹은 음식이 든든해서 오후까지 배가 부르네요.',
    hint: '맛이나 식감 중 떠오르는 것 하나만 이야기해도 좋아요.',
  },
  {
    key: 'everyday_food_07',
    part: 'description',
    photo: false,
    starter: false,
    text: '한입 먹었을 때의 느낌을 친구에게 전해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '처음 먹어 본 과일이 생각보다 새콤해요.',
    hint: '맛이나 식감 중 떠오르는 것 하나만 이야기해도 좋아요.',
  },
  {
    key: 'everyday_food_08',
    part: 'description',
    photo: false,
    starter: false,
    text: '한입 먹었을 때의 느낌을 친구에게 전해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '식은 음식도 데워 먹으니 다시 맛있어졌어요.',
    hint: '맛이나 식감 중 떠오르는 것 하나만 이야기해도 좋아요.',
  },
  {
    key: 'everyday_drink_01',
    part: 'description',
    photo: false,
    starter: false,
    text: '지금 마신 음료를 친구에게 소개해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '추운 날 따뜻한 차 한 잔을 손에 쥐었어요.',
    hint: '향이나 온도, 기분을 편하게 말해 주세요.',
  },
  {
    key: 'everyday_drink_02',
    part: 'description',
    photo: false,
    starter: false,
    text: '지금 마신 음료를 친구에게 소개해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '더운 날 차가운 물을 마시니 갈증이 가셔요.',
    hint: '향이나 온도, 기분을 편하게 말해 주세요.',
  },
  {
    key: 'everyday_drink_03',
    part: 'description',
    photo: false,
    starter: false,
    text: '지금 마신 음료를 친구에게 소개해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '늘 마시던 음료 대신 다른 맛을 골라 봤어요.',
    hint: '향이나 온도, 기분을 편하게 말해 주세요.',
  },
  {
    key: 'everyday_drink_04',
    part: 'description',
    photo: false,
    starter: false,
    text: '지금 마신 음료를 친구에게 소개해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '카페에서 받은 음료가 생각보다 진해요.',
    hint: '향이나 온도, 기분을 편하게 말해 주세요.',
  },
  {
    key: 'everyday_drink_05',
    part: 'description',
    photo: false,
    starter: false,
    text: '지금 마신 음료를 친구에게 소개해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '달지 않을 줄 알았던 음료가 꽤 달아요.',
    hint: '향이나 온도, 기분을 편하게 말해 주세요.',
  },
  {
    key: 'everyday_drink_06',
    part: 'description',
    photo: false,
    starter: false,
    text: '지금 마신 음료를 친구에게 소개해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '집에서 만든 음료를 얼음 넣어 마셔 봤어요.',
    hint: '향이나 온도, 기분을 편하게 말해 주세요.',
  },
  {
    key: 'everyday_drink_07',
    part: 'description',
    photo: false,
    starter: false,
    text: '지금 마신 음료를 친구에게 소개해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '창가에 앉아 천천히 따뜻한 음료를 마시고 있어요.',
    hint: '향이나 온도, 기분을 편하게 말해 주세요.',
  },
  {
    key: 'everyday_drink_08',
    part: 'description',
    photo: false,
    starter: false,
    text: '지금 마신 음료를 친구에게 소개해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '친구가 고른 음료를 한 모금 맛보니 취향이 달라요.',
    hint: '향이나 온도, 기분을 편하게 말해 주세요.',
  },
  {
    key: 'everyday_commute_01',
    part: 'description',
    photo: false,
    starter: false,
    text: '길에서 느낀 점을 친구에게 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '버스를 놓쳤는데 다음 버스가 금방 왔어요.',
    hint: '불편함이나 작은 즐거움 중 하나만 골라도 좋아요.',
  },
  {
    key: 'everyday_commute_02',
    part: 'description',
    photo: false,
    starter: false,
    text: '길에서 느낀 점을 친구에게 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '지하철에서 운 좋게 자리에 앉았어요.',
    hint: '불편함이나 작은 즐거움 중 하나만 골라도 좋아요.',
  },
  {
    key: 'everyday_commute_03',
    part: 'description',
    photo: false,
    starter: false,
    text: '길에서 느낀 점을 친구에게 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '늘 가던 길 대신 다른 골목으로 걸어 봤어요.',
    hint: '불편함이나 작은 즐거움 중 하나만 골라도 좋아요.',
  },
  {
    key: 'everyday_commute_04',
    part: 'description',
    photo: false,
    starter: false,
    text: '길에서 느낀 점을 친구에게 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '이어폰 없이 걸으니 주변 소리가 잘 들려요.',
    hint: '불편함이나 작은 즐거움 중 하나만 골라도 좋아요.',
  },
  {
    key: 'everyday_commute_05',
    part: 'description',
    photo: false,
    starter: false,
    text: '길에서 느낀 점을 친구에게 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '횡단보도 신호가 바로 바뀌어서 덜 기다렸어요.',
    hint: '불편함이나 작은 즐거움 중 하나만 골라도 좋아요.',
  },
  {
    key: 'everyday_commute_06',
    part: 'description',
    photo: false,
    starter: false,
    text: '길에서 느낀 점을 친구에게 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '가까운 거리는 걸어 보니 생각보다 빨리 도착했어요.',
    hint: '불편함이나 작은 즐거움 중 하나만 골라도 좋아요.',
  },
  {
    key: 'everyday_commute_07',
    part: 'description',
    photo: false,
    starter: false,
    text: '길에서 느낀 점을 친구에게 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '길을 잘못 들었지만 예쁜 나무를 발견했어요.',
    hint: '불편함이나 작은 즐거움 중 하나만 골라도 좋아요.',
  },
  {
    key: 'everyday_commute_08',
    part: 'description',
    photo: false,
    starter: false,
    text: '길에서 느낀 점을 친구에게 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '붐비는 시간대를 피하니 이동이 한결 편해요.',
    hint: '불편함이나 작은 즐거움 중 하나만 골라도 좋아요.',
  },
  {
    key: 'everyday_shopping_01',
    part: 'description',
    photo: false,
    starter: false,
    text: '이 물건을 고른 마음을 친구에게 알려 주세요. 1~3문장으로 편하게 써 주세요.',
    scene: '필요한 물건을 사러 갔다가 색깔 때문에 고민 중이에요.',
    hint: '써 보기 전 기대나 써 본 느낌도 좋아요.',
  },
  {
    key: 'everyday_shopping_02',
    part: 'description',
    photo: false,
    starter: false,
    text: '이 물건을 고른 마음을 친구에게 알려 주세요. 1~3문장으로 편하게 써 주세요.',
    scene: '세일하는 물건을 봤지만 당장 필요하지는 않아요.',
    hint: '써 보기 전 기대나 써 본 느낌도 좋아요.',
  },
  {
    key: 'everyday_shopping_03',
    part: 'description',
    photo: false,
    starter: false,
    text: '이 물건을 고른 마음을 친구에게 알려 주세요. 1~3문장으로 편하게 써 주세요.',
    scene: '온라인 사진보다 실제 물건의 색이 더 마음에 들어요.',
    hint: '써 보기 전 기대나 써 본 느낌도 좋아요.',
  },
  {
    key: 'everyday_shopping_04',
    part: 'description',
    photo: false,
    starter: false,
    text: '이 물건을 고른 마음을 친구에게 알려 주세요. 1~3문장으로 편하게 써 주세요.',
    scene: '가격은 조금 높지만 오래 쓸 것 같은 물건을 발견했어요.',
    hint: '써 보기 전 기대나 써 본 느낌도 좋아요.',
  },
  {
    key: 'everyday_shopping_05',
    part: 'description',
    photo: false,
    starter: false,
    text: '이 물건을 고른 마음을 친구에게 알려 주세요. 1~3문장으로 편하게 써 주세요.',
    scene: '작은 수납함 하나로 책상이 깔끔해졌어요.',
    hint: '써 보기 전 기대나 써 본 느낌도 좋아요.',
  },
  {
    key: 'everyday_shopping_06',
    part: 'description',
    photo: false,
    starter: false,
    text: '이 물건을 고른 마음을 친구에게 알려 주세요. 1~3문장으로 편하게 써 주세요.',
    scene: '편할 줄 알고 산 신발이 처음엔 조금 뻣뻣해요.',
    hint: '써 보기 전 기대나 써 본 느낌도 좋아요.',
  },
  {
    key: 'everyday_shopping_07',
    part: 'description',
    photo: false,
    starter: false,
    text: '이 물건을 고른 마음을 친구에게 알려 주세요. 1~3문장으로 편하게 써 주세요.',
    scene: '장바구니에 담아 뒀던 물건을 다시 보니 마음이 바뀌어요.',
    hint: '써 보기 전 기대나 써 본 느낌도 좋아요.',
  },
  {
    key: 'everyday_shopping_08',
    part: 'description',
    photo: false,
    starter: false,
    text: '이 물건을 고른 마음을 친구에게 알려 주세요. 1~3문장으로 편하게 써 주세요.',
    scene: '포장이 단순해서 물건을 꺼내기 편했어요.',
    hint: '써 보기 전 기대나 써 본 느낌도 좋아요.',
  },
  {
    key: 'everyday_gift_01',
    part: 'description',
    photo: false,
    starter: false,
    text: '상대에게 어떤 말을 건네고 싶은지 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '친구에게 간식 하나를 가볍게 건네려 해요.',
    hint: '비싼 선물이나 특별한 날을 떠올릴 필요는 없어요.',
  },
  {
    key: 'everyday_gift_02',
    part: 'description',
    photo: false,
    starter: false,
    text: '상대에게 어떤 말을 건네고 싶은지 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '작은 쪽지를 선물에 함께 넣고 싶어요.',
    hint: '비싼 선물이나 특별한 날을 떠올릴 필요는 없어요.',
  },
  {
    key: 'everyday_gift_03',
    part: 'description',
    photo: false,
    starter: false,
    text: '상대에게 어떤 말을 건네고 싶은지 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '친구가 좋아하는 색의 물건을 우연히 발견했어요.',
    hint: '비싼 선물이나 특별한 날을 떠올릴 필요는 없어요.',
  },
  {
    key: 'everyday_gift_04',
    part: 'description',
    photo: false,
    starter: false,
    text: '상대에게 어떤 말을 건네고 싶은지 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '뜻밖에 작은 선물을 받아서 기분이 좋아요.',
    hint: '비싼 선물이나 특별한 날을 떠올릴 필요는 없어요.',
  },
  {
    key: 'everyday_gift_05',
    part: 'description',
    photo: false,
    starter: false,
    text: '상대에게 어떤 말을 건네고 싶은지 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '선물을 고르다가 내 취향과 친구 취향이 다르다는 걸 느껴요.',
    hint: '비싼 선물이나 특별한 날을 떠올릴 필요는 없어요.',
  },
  {
    key: 'everyday_gift_06',
    part: 'description',
    photo: false,
    starter: false,
    text: '상대에게 어떤 말을 건네고 싶은지 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '친구가 준 물건을 잘 쓰고 있다는 말을 전하려 해요.',
    hint: '비싼 선물이나 특별한 날을 떠올릴 필요는 없어요.',
  },
  {
    key: 'everyday_gift_07',
    part: 'description',
    photo: false,
    starter: false,
    text: '상대에게 어떤 말을 건네고 싶은지 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '직접 만든 작은 것을 누군가에게 보여주고 싶어요.',
    hint: '비싼 선물이나 특별한 날을 떠올릴 필요는 없어요.',
  },
  {
    key: 'everyday_gift_08',
    part: 'description',
    photo: false,
    starter: false,
    text: '상대에게 어떤 말을 건네고 싶은지 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '친구가 선물은 필요 없다며 같이 시간을 보내자고 해요.',
    hint: '비싼 선물이나 특별한 날을 떠올릴 필요는 없어요.',
  },
  {
    key: 'everyday_home_01',
    part: 'description',
    photo: false,
    starter: false,
    text: '집에서 생긴 작은 변화를 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '창문을 열었더니 시원한 바람이 들어와요.',
    hint: '집이 크거나 특별할 필요는 없어요.',
  },
  {
    key: 'everyday_home_02',
    part: 'description',
    photo: false,
    starter: false,
    text: '집에서 생긴 작은 변화를 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '밀린 빨래를 끝내니 마음이 조금 가벼워졌어요.',
    hint: '집이 크거나 특별할 필요는 없어요.',
  },
  {
    key: 'everyday_home_03',
    part: 'description',
    photo: false,
    starter: false,
    text: '집에서 생긴 작은 변화를 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '가구를 조금 옮겼더니 방 분위기가 달라졌어요.',
    hint: '집이 크거나 특별할 필요는 없어요.',
  },
  {
    key: 'everyday_home_04',
    part: 'description',
    photo: false,
    starter: false,
    text: '집에서 생긴 작은 변화를 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '책상 위 물건을 정리하다 잊고 있던 메모를 찾았어요.',
    hint: '집이 크거나 특별할 필요는 없어요.',
  },
  {
    key: 'everyday_home_05',
    part: 'description',
    photo: false,
    starter: false,
    text: '집에서 생긴 작은 변화를 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '불을 조금 낮추니 저녁이 더 편안하게 느껴져요.',
    hint: '집이 크거나 특별할 필요는 없어요.',
  },
  {
    key: 'everyday_home_06',
    part: 'description',
    photo: false,
    starter: false,
    text: '집에서 생긴 작은 변화를 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '새로 꺼낸 이불이 보송해서 기분이 좋아요.',
    hint: '집이 크거나 특별할 필요는 없어요.',
  },
  {
    key: 'everyday_home_07',
    part: 'description',
    photo: false,
    starter: false,
    text: '집에서 생긴 작은 변화를 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '청소를 시작하기 전에는 귀찮았는데 끝내니 뿌듯해요.',
    hint: '집이 크거나 특별할 필요는 없어요.',
  },
  {
    key: 'everyday_home_08',
    part: 'description',
    photo: false,
    starter: false,
    text: '집에서 생긴 작은 변화를 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '집 안에 둔 작은 화분에서 새잎이 나왔어요.',
    hint: '집이 크거나 특별할 필요는 없어요.',
  },
  {
    key: 'everyday_weekend_01',
    part: 'description',
    photo: false,
    starter: false,
    text: '이 시간을 어떻게 보내고 싶은지 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '주말에 알람 없이 조금 늦게 일어났어요.',
    hint: '계획 없이 쉬는 것도 좋은 답이에요.',
  },
  {
    key: 'everyday_weekend_02',
    part: 'description',
    photo: false,
    starter: false,
    text: '이 시간을 어떻게 보내고 싶은지 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '오늘은 약속이 없어서 천천히 아침을 먹어요.',
    hint: '계획 없이 쉬는 것도 좋은 답이에요.',
  },
  {
    key: 'everyday_weekend_03',
    part: 'description',
    photo: false,
    starter: false,
    text: '이 시간을 어떻게 보내고 싶은지 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '잠깐 나갔다 오려다가 집이 더 편하게 느껴져요.',
    hint: '계획 없이 쉬는 것도 좋은 답이에요.',
  },
  {
    key: 'everyday_weekend_04',
    part: 'description',
    photo: false,
    starter: false,
    text: '이 시간을 어떻게 보내고 싶은지 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '평소 미뤄 둔 작은 일을 주말에 하나 끝냈어요.',
    hint: '계획 없이 쉬는 것도 좋은 답이에요.',
  },
  {
    key: 'everyday_weekend_05',
    part: 'description',
    photo: false,
    starter: false,
    text: '이 시간을 어떻게 보내고 싶은지 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '산책을 하려는데 친구가 같이 걷자고 연락했어요.',
    hint: '계획 없이 쉬는 것도 좋은 답이에요.',
  },
  {
    key: 'everyday_weekend_06',
    part: 'description',
    photo: false,
    starter: false,
    text: '이 시간을 어떻게 보내고 싶은지 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '낮잠을 짧게 자려고 했는데 생각보다 오래 잤어요.',
    hint: '계획 없이 쉬는 것도 좋은 답이에요.',
  },
  {
    key: 'everyday_weekend_07',
    part: 'description',
    photo: false,
    starter: false,
    text: '이 시간을 어떻게 보내고 싶은지 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '동네만 가볍게 둘러봤는데 기분이 좋아졌어요.',
    hint: '계획 없이 쉬는 것도 좋은 답이에요.',
  },
  {
    key: 'everyday_weekend_08',
    part: 'description',
    photo: false,
    starter: false,
    text: '이 시간을 어떻게 보내고 싶은지 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '다음 주를 위해 조금 준비할지 더 쉴지 고민돼요.',
    hint: '계획 없이 쉬는 것도 좋은 답이에요.',
  },
  {
    key: 'everyday_weather_01',
    part: 'description',
    photo: false,
    starter: false,
    text: '날씨가 바꾼 기분을 친구에게 전해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '아침에 햇빛이 밝아서 커튼을 활짝 열었어요.',
    hint: '풍경을 길게 묘사하지 않아도 괜찮아요.',
  },
  {
    key: 'everyday_weather_02',
    part: 'description',
    photo: false,
    starter: false,
    text: '날씨가 바꾼 기분을 친구에게 전해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '갑자기 바람이 차가워져 겉옷을 챙겼어요.',
    hint: '풍경을 길게 묘사하지 않아도 괜찮아요.',
  },
  {
    key: 'everyday_weather_03',
    part: 'description',
    photo: false,
    starter: false,
    text: '날씨가 바꾼 기분을 친구에게 전해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '비가 그친 뒤 공기가 맑게 느껴져요.',
    hint: '풍경을 길게 묘사하지 않아도 괜찮아요.',
  },
  {
    key: 'everyday_weather_04',
    part: 'description',
    photo: false,
    starter: false,
    text: '날씨가 바꾼 기분을 친구에게 전해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '눈이 조금 내려서 평소 길이 새롭게 보여요.',
    hint: '풍경을 길게 묘사하지 않아도 괜찮아요.',
  },
  {
    key: 'everyday_weather_05',
    part: 'description',
    photo: false,
    starter: false,
    text: '날씨가 바꾼 기분을 친구에게 전해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '더운 오후에 그늘을 찾으니 한결 시원해요.',
    hint: '풍경을 길게 묘사하지 않아도 괜찮아요.',
  },
  {
    key: 'everyday_weather_06',
    part: 'description',
    photo: false,
    starter: false,
    text: '날씨가 바꾼 기분을 친구에게 전해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '흐린 날이라 괜히 느긋하게 쉬고 싶어요.',
    hint: '풍경을 길게 묘사하지 않아도 괜찮아요.',
  },
  {
    key: 'everyday_weather_07',
    part: 'description',
    photo: false,
    starter: false,
    text: '날씨가 바꾼 기분을 친구에게 전해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '저녁 바람이 선선해서 조금 더 걷고 싶어요.',
    hint: '풍경을 길게 묘사하지 않아도 괜찮아요.',
  },
  {
    key: 'everyday_weather_08',
    part: 'description',
    photo: false,
    starter: false,
    text: '날씨가 바꾼 기분을 친구에게 전해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '날씨가 좋아서 예정에 없던 산책을 하려 해요.',
    hint: '풍경을 길게 묘사하지 않아도 괜찮아요.',
  },
  {
    key: 'everyday_plans_01',
    part: 'description',
    photo: false,
    starter: false,
    text: '계획을 정하면서 든 생각을 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '하려던 일이 일찍 끝나서 시간이 조금 남았어요.',
    hint: '작은 선택의 이유면 충분해요.',
  },
  {
    key: 'everyday_plans_02',
    part: 'description',
    photo: false,
    starter: false,
    text: '계획을 정하면서 든 생각을 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '내일 해야 할 일을 세 가지 적어 봤어요.',
    hint: '작은 선택의 이유면 충분해요.',
  },
  {
    key: 'everyday_plans_03',
    part: 'description',
    photo: false,
    starter: false,
    text: '계획을 정하면서 든 생각을 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '한 번에 다 하려니 부담돼서 하나만 먼저 하기로 했어요.',
    hint: '작은 선택의 이유면 충분해요.',
  },
  {
    key: 'everyday_plans_04',
    part: 'description',
    photo: false,
    starter: false,
    text: '계획을 정하면서 든 생각을 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '약속 시간을 조금 늦추자는 연락을 받았어요.',
    hint: '작은 선택의 이유면 충분해요.',
  },
  {
    key: 'everyday_plans_05',
    part: 'description',
    photo: false,
    starter: false,
    text: '계획을 정하면서 든 생각을 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '밖에서 만나려던 계획을 비 때문에 실내로 바꿨어요.',
    hint: '작은 선택의 이유면 충분해요.',
  },
  {
    key: 'everyday_plans_06',
    part: 'description',
    photo: false,
    starter: false,
    text: '계획을 정하면서 든 생각을 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '오늘 계획에서 가장 하기 싫은 일을 먼저 끝냈어요.',
    hint: '작은 선택의 이유면 충분해요.',
  },
  {
    key: 'everyday_plans_07',
    part: 'description',
    photo: false,
    starter: false,
    text: '계획을 정하면서 든 생각을 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '어디에 갈지 정하지 않고 걷기로 했어요.',
    hint: '작은 선택의 이유면 충분해요.',
  },
  {
    key: 'everyday_plans_08',
    part: 'description',
    photo: false,
    starter: false,
    text: '계획을 정하면서 든 생각을 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '계획을 너무 꽉 채운 것 같아 하나를 빼려 해요.',
    hint: '작은 선택의 이유면 충분해요.',
  },
  {
    key: 'everyday_friends_01',
    part: 'description',
    photo: false,
    starter: false,
    text: '친구에게 건넬 말을 평소처럼 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '친구가 새로운 취미를 시작했다며 이야기해 줬어요.',
    hint: '격식을 차리지 않아도 괜찮아요.',
  },
  {
    key: 'everyday_friends_02',
    part: 'description',
    photo: false,
    starter: false,
    text: '친구에게 건넬 말을 평소처럼 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '친구가 별일 아닌 실수를 해서 속상해해요.',
    hint: '격식을 차리지 않아도 괜찮아요.',
  },
  {
    key: 'everyday_friends_03',
    part: 'description',
    photo: false,
    starter: false,
    text: '친구에게 건넬 말을 평소처럼 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '친구와 같은 일을 두고 다른 생각을 하고 있어요.',
    hint: '격식을 차리지 않아도 괜찮아요.',
  },
  {
    key: 'everyday_friends_04',
    part: 'description',
    photo: false,
    starter: false,
    text: '친구에게 건넬 말을 평소처럼 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '친구가 바쁜 하루를 보냈다며 지쳤다고 해요.',
    hint: '격식을 차리지 않아도 괜찮아요.',
  },
  {
    key: 'everyday_friends_05',
    part: 'description',
    photo: false,
    starter: false,
    text: '친구에게 건넬 말을 평소처럼 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '친구가 맛있는 간식을 발견했다며 추천해 줬어요.',
    hint: '격식을 차리지 않아도 괜찮아요.',
  },
  {
    key: 'everyday_friends_06',
    part: 'description',
    photo: false,
    starter: false,
    text: '친구에게 건넬 말을 평소처럼 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '친구가 약속에 조금 늦는다고 먼저 연락했어요.',
    hint: '격식을 차리지 않아도 괜찮아요.',
  },
  {
    key: 'everyday_friends_07',
    part: 'description',
    photo: false,
    starter: false,
    text: '친구에게 건넬 말을 평소처럼 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '친구와 오랜만에 짧은 메시지를 주고받았어요.',
    hint: '격식을 차리지 않아도 괜찮아요.',
  },
  {
    key: 'everyday_friends_08',
    part: 'description',
    photo: false,
    starter: false,
    text: '친구에게 건넬 말을 평소처럼 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '친구가 작은 목표를 이뤘다며 기뻐해요.',
    hint: '격식을 차리지 않아도 괜찮아요.',
  },
  {
    key: 'everyday_surprises_01',
    part: 'description',
    photo: false,
    starter: false,
    text: '뜻밖의 순간에 든 마음을 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '일기예보와 달리 산책하는 동안 비가 오지 않았어요.',
    hint: '좋거나 아쉬운 감정을 자연스럽게 말해 주세요.',
  },
  {
    key: 'everyday_surprises_02',
    part: 'description',
    photo: false,
    starter: false,
    text: '뜻밖의 순간에 든 마음을 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '잃어버린 줄 알았던 물건이 가방 안에 있었어요.',
    hint: '좋거나 아쉬운 감정을 자연스럽게 말해 주세요.',
  },
  {
    key: 'everyday_surprises_03',
    part: 'description',
    photo: false,
    starter: false,
    text: '뜻밖의 순간에 든 마음을 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '별 기대 없이 본 영상이 웃겨서 계속 웃었어요.',
    hint: '좋거나 아쉬운 감정을 자연스럽게 말해 주세요.',
  },
  {
    key: 'everyday_surprises_04',
    part: 'description',
    photo: false,
    starter: false,
    text: '뜻밖의 순간에 든 마음을 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '집에 오는 길에 마음에 드는 노래를 들었어요.',
    hint: '좋거나 아쉬운 감정을 자연스럽게 말해 주세요.',
  },
  {
    key: 'everyday_surprises_05',
    part: 'description',
    photo: false,
    starter: false,
    text: '뜻밖의 순간에 든 마음을 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '자주 보던 가게가 새로 단장했어요.',
    hint: '좋거나 아쉬운 감정을 자연스럽게 말해 주세요.',
  },
  {
    key: 'everyday_surprises_06',
    part: 'description',
    photo: false,
    starter: false,
    text: '뜻밖의 순간에 든 마음을 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '평소 조용하던 골목에 사람들이 모여 있어요.',
    hint: '좋거나 아쉬운 감정을 자연스럽게 말해 주세요.',
  },
  {
    key: 'everyday_surprises_07',
    part: 'description',
    photo: false,
    starter: false,
    text: '뜻밖의 순간에 든 마음을 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '주문한 물건이 예상보다 하루 일찍 도착했어요.',
    hint: '좋거나 아쉬운 감정을 자연스럽게 말해 주세요.',
  },
  {
    key: 'everyday_surprises_08',
    part: 'description',
    photo: false,
    starter: false,
    text: '뜻밖의 순간에 든 마음을 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '어렵게 느껴지던 일이 해 보니 생각보다 간단했어요.',
    hint: '좋거나 아쉬운 감정을 자연스럽게 말해 주세요.',
  },
  {
    key: 'everyday_waiting_01',
    part: 'description',
    photo: false,
    starter: false,
    text: '기다리는 동안의 마음을 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '주문한 음식이 평소보다 조금 늦게 나와요.',
    hint: '화가 나지 않았다면 그 이유를 써도 좋아요.',
  },
  {
    key: 'everyday_waiting_02',
    part: 'description',
    photo: false,
    starter: false,
    text: '기다리는 동안의 마음을 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '엘리베이터를 기다리다 계단을 이용하기로 했어요.',
    hint: '화가 나지 않았다면 그 이유를 써도 좋아요.',
  },
  {
    key: 'everyday_waiting_03',
    part: 'description',
    photo: false,
    starter: false,
    text: '기다리는 동안의 마음을 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '버스 도착 시간이 몇 분씩 밀리고 있어요.',
    hint: '화가 나지 않았다면 그 이유를 써도 좋아요.',
  },
  {
    key: 'everyday_waiting_04',
    part: 'description',
    photo: false,
    starter: false,
    text: '기다리는 동안의 마음을 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '친구를 기다리는 동안 근처를 구경했어요.',
    hint: '화가 나지 않았다면 그 이유를 써도 좋아요.',
  },
  {
    key: 'everyday_waiting_05',
    part: 'description',
    photo: false,
    starter: false,
    text: '기다리는 동안의 마음을 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '온라인 주문의 배송 알림이 늦게 왔어요.',
    hint: '화가 나지 않았다면 그 이유를 써도 좋아요.',
  },
  {
    key: 'everyday_waiting_06',
    part: 'description',
    photo: false,
    starter: false,
    text: '기다리는 동안의 마음을 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '가게 앞에 줄이 길어서 다른 곳에 가려 해요.',
    hint: '화가 나지 않았다면 그 이유를 써도 좋아요.',
  },
  {
    key: 'everyday_waiting_07',
    part: 'description',
    photo: false,
    starter: false,
    text: '기다리는 동안의 마음을 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '예약 시간보다 일찍 도착해서 잠깐 쉬고 있어요.',
    hint: '화가 나지 않았다면 그 이유를 써도 좋아요.',
  },
  {
    key: 'everyday_waiting_08',
    part: 'description',
    photo: false,
    starter: false,
    text: '기다리는 동안의 마음을 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '급하지 않은 날이라 기다림이 크게 불편하지 않아요.',
    hint: '화가 나지 않았다면 그 이유를 써도 좋아요.',
  },
  {
    key: 'everyday_choices_01',
    part: 'description',
    photo: false,
    starter: false,
    text: '무엇을 선택하고 싶은지 이유를 알려 주세요. 1~3문장으로 편하게 써 주세요.',
    scene: '가까운 곳에서 편하게 쉴지 조금 멀리 나갈지 고민돼요.',
    hint: '선택에는 정답이 없어요.',
  },
  {
    key: 'everyday_choices_02',
    part: 'description',
    photo: false,
    starter: false,
    text: '무엇을 선택하고 싶은지 이유를 알려 주세요. 1~3문장으로 편하게 써 주세요.',
    scene: '간단한 간식과 든든한 식사 사이에서 고민 중이에요.',
    hint: '선택에는 정답이 없어요.',
  },
  {
    key: 'everyday_choices_03',
    part: 'description',
    photo: false,
    starter: false,
    text: '무엇을 선택하고 싶은지 이유를 알려 주세요. 1~3문장으로 편하게 써 주세요.',
    scene: '오늘은 조용한 음악과 신나는 음악 중 하나를 듣고 싶어요.',
    hint: '선택에는 정답이 없어요.',
  },
  {
    key: 'everyday_choices_04',
    part: 'description',
    photo: false,
    starter: false,
    text: '무엇을 선택하고 싶은지 이유를 알려 주세요. 1~3문장으로 편하게 써 주세요.',
    scene: '물건을 바로 살지 며칠 더 생각할지 고민돼요.',
    hint: '선택에는 정답이 없어요.',
  },
  {
    key: 'everyday_choices_05',
    part: 'description',
    photo: false,
    starter: false,
    text: '무엇을 선택하고 싶은지 이유를 알려 주세요. 1~3문장으로 편하게 써 주세요.',
    scene: '친구를 만나거나 혼자 시간을 보내는 둘 다 좋아 보여요.',
    hint: '선택에는 정답이 없어요.',
  },
  {
    key: 'everyday_choices_06',
    part: 'description',
    photo: false,
    starter: false,
    text: '무엇을 선택하고 싶은지 이유를 알려 주세요. 1~3문장으로 편하게 써 주세요.',
    scene: '편한 옷과 조금 차려입는 옷 사이에서 고민돼요.',
    hint: '선택에는 정답이 없어요.',
  },
  {
    key: 'everyday_choices_07',
    part: 'description',
    photo: false,
    starter: false,
    text: '무엇을 선택하고 싶은지 이유를 알려 주세요. 1~3문장으로 편하게 써 주세요.',
    scene: '새로운 길로 가거나 익숙한 길로 돌아갈 수 있어요.',
    hint: '선택에는 정답이 없어요.',
  },
  {
    key: 'everyday_choices_08',
    part: 'description',
    photo: false,
    starter: false,
    text: '무엇을 선택하고 싶은지 이유를 알려 주세요. 1~3문장으로 편하게 써 주세요.',
    scene: '사진을 찍을지 그냥 풍경을 보고 있을지 고민돼요.',
    hint: '선택에는 정답이 없어요.',
  },
  {
    key: 'everyday_comfort_01',
    part: 'description',
    photo: false,
    starter: false,
    text: '마음이 편해진 이유를 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '해야 할 일을 끝내고 따뜻한 음료를 마셔요.',
    hint: '대단한 일이 없어도 괜찮아요.',
  },
  {
    key: 'everyday_comfort_02',
    part: 'description',
    photo: false,
    starter: false,
    text: '마음이 편해진 이유를 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '친구가 괜찮다는 한마디를 건네 줬어요.',
    hint: '대단한 일이 없어도 괜찮아요.',
  },
  {
    key: 'everyday_comfort_03',
    part: 'description',
    photo: false,
    starter: false,
    text: '마음이 편해진 이유를 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '잠깐 눈을 감고 쉬었더니 머리가 맑아져요.',
    hint: '대단한 일이 없어도 괜찮아요.',
  },
  {
    key: 'everyday_comfort_04',
    part: 'description',
    photo: false,
    starter: false,
    text: '마음이 편해진 이유를 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '마음에 드는 노래를 천천히 다시 듣고 있어요.',
    hint: '대단한 일이 없어도 괜찮아요.',
  },
  {
    key: 'everyday_comfort_05',
    part: 'description',
    photo: false,
    starter: false,
    text: '마음이 편해진 이유를 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '비가 오는 소리를 들으며 집에서 쉬고 있어요.',
    hint: '대단한 일이 없어도 괜찮아요.',
  },
  {
    key: 'everyday_comfort_06',
    part: 'description',
    photo: false,
    starter: false,
    text: '마음이 편해진 이유를 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '정리된 책상을 보니 다시 시작할 힘이 나요.',
    hint: '대단한 일이 없어도 괜찮아요.',
  },
  {
    key: 'everyday_comfort_07',
    part: 'description',
    photo: false,
    starter: false,
    text: '마음이 편해진 이유를 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '천천히 걷다 보니 복잡했던 생각이 줄었어요.',
    hint: '대단한 일이 없어도 괜찮아요.',
  },
  {
    key: 'everyday_comfort_08',
    part: 'description',
    photo: false,
    starter: false,
    text: '마음이 편해진 이유를 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '별말 없이 함께 있어 주는 친구가 고마워요.',
    hint: '대단한 일이 없어도 괜찮아요.',
  },
  {
    key: 'everyday_everyday_app_01',
    part: 'description',
    photo: false,
    starter: false,
    text: '이 기능을 써 본 느낌을 쉽게 설명해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '메모를 휴대폰에 적어 두니 잊지 않아서 편해요.',
    hint: '앱 이름이나 기술 용어를 몰라도 괜찮아요.',
  },
  {
    key: 'everyday_everyday_app_02',
    part: 'description',
    photo: false,
    starter: false,
    text: '이 기능을 써 본 느낌을 쉽게 설명해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '앱 알림이 너무 자주 와서 몇 개를 껐어요.',
    hint: '앱 이름이나 기술 용어를 몰라도 괜찮아요.',
  },
  {
    key: 'everyday_everyday_app_03',
    part: 'description',
    photo: false,
    starter: false,
    text: '이 기능을 써 본 느낌을 쉽게 설명해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '글자가 작아서 화면을 조금 확대했어요.',
    hint: '앱 이름이나 기술 용어를 몰라도 괜찮아요.',
  },
  {
    key: 'everyday_everyday_app_04',
    part: 'description',
    photo: false,
    starter: false,
    text: '이 기능을 써 본 느낌을 쉽게 설명해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '검색하던 정보를 생각보다 쉽게 찾았어요.',
    hint: '앱 이름이나 기술 용어를 몰라도 괜찮아요.',
  },
  {
    key: 'everyday_everyday_app_05',
    part: 'description',
    photo: false,
    starter: false,
    text: '이 기능을 써 본 느낌을 쉽게 설명해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '평소 쓰던 버튼 위치가 바뀌어 잠깐 헤맸어요.',
    hint: '앱 이름이나 기술 용어를 몰라도 괜찮아요.',
  },
  {
    key: 'everyday_everyday_app_06',
    part: 'description',
    photo: false,
    starter: false,
    text: '이 기능을 써 본 느낌을 쉽게 설명해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '사진을 한 폴더에 모으니 찾기가 쉬워졌어요.',
    hint: '앱 이름이나 기술 용어를 몰라도 괜찮아요.',
  },
  {
    key: 'everyday_everyday_app_07',
    part: 'description',
    photo: false,
    starter: false,
    text: '이 기능을 써 본 느낌을 쉽게 설명해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '앱에서 하던 일이 저장돼 있어서 안심했어요.',
    hint: '앱 이름이나 기술 용어를 몰라도 괜찮아요.',
  },
  {
    key: 'everyday_everyday_app_08',
    part: 'description',
    photo: false,
    starter: false,
    text: '이 기능을 써 본 느낌을 쉽게 설명해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '설정이 많아서 필요한 것만 먼저 골랐어요.',
    hint: '앱 이름이나 기술 용어를 몰라도 괜찮아요.',
  },
  {
    key: 'everyday_stories_01',
    part: 'description',
    photo: false,
    starter: false,
    text: '짧은 이야기를 본 느낌을 친구에게 전해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '짧은 이야기를 읽었는데 마지막 문장이 오래 남아요.',
    hint: '작품 제목이나 줄거리를 정확히 쓸 필요는 없어요.',
  },
  {
    key: 'everyday_stories_02',
    part: 'description',
    photo: false,
    starter: false,
    text: '짧은 이야기를 본 느낌을 친구에게 전해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '재미있는 장면을 보고 친구에게 보여주고 싶어요.',
    hint: '작품 제목이나 줄거리를 정확히 쓸 필요는 없어요.',
  },
  {
    key: 'everyday_stories_03',
    part: 'description',
    photo: false,
    starter: false,
    text: '짧은 이야기를 본 느낌을 친구에게 전해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '예상하던 결말과 달라서 조금 놀랐어요.',
    hint: '작품 제목이나 줄거리를 정확히 쓸 필요는 없어요.',
  },
  {
    key: 'everyday_stories_04',
    part: 'description',
    photo: false,
    starter: false,
    text: '짧은 이야기를 본 느낌을 친구에게 전해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '쉬려고 영상을 봤는데 생각할 거리가 생겼어요.',
    hint: '작품 제목이나 줄거리를 정확히 쓸 필요는 없어요.',
  },
  {
    key: 'everyday_stories_05',
    part: 'description',
    photo: false,
    starter: false,
    text: '짧은 이야기를 본 느낌을 친구에게 전해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '짧은 글 하나가 오늘 기분과 잘 맞아요.',
    hint: '작품 제목이나 줄거리를 정확히 쓸 필요는 없어요.',
  },
  {
    key: 'everyday_stories_06',
    part: 'description',
    photo: false,
    starter: false,
    text: '짧은 이야기를 본 느낌을 친구에게 전해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '다시 본 장면이 처음 볼 때와 다르게 느껴져요.',
    hint: '작품 제목이나 줄거리를 정확히 쓸 필요는 없어요.',
  },
  {
    key: 'everyday_stories_07',
    part: 'description',
    photo: false,
    starter: false,
    text: '짧은 이야기를 본 느낌을 친구에게 전해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '내용은 좋지만 설명이 길어서 조금 지루해요.',
    hint: '작품 제목이나 줄거리를 정확히 쓸 필요는 없어요.',
  },
  {
    key: 'everyday_stories_08',
    part: 'description',
    photo: false,
    starter: false,
    text: '짧은 이야기를 본 느낌을 친구에게 전해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '가볍게 보기 시작했는데 끝까지 보고 싶어졌어요.',
    hint: '작품 제목이나 줄거리를 정확히 쓸 필요는 없어요.',
  },
  {
    key: 'everyday_photos_01',
    part: 'description',
    photo: false,
    starter: false,
    text: '사진을 찍거나 본 마음을 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '노을 색이 예뻐서 휴대폰으로 한 장 찍었어요.',
    hint: '사진을 올릴 필요 없이 상황만 상상하면 돼요.',
  },
  {
    key: 'everyday_photos_02',
    part: 'description',
    photo: false,
    starter: false,
    text: '사진을 찍거나 본 마음을 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '사진보다 실제 풍경이 훨씬 멋지게 느껴져요.',
    hint: '사진을 올릴 필요 없이 상황만 상상하면 돼요.',
  },
  {
    key: 'everyday_photos_03',
    part: 'description',
    photo: false,
    starter: false,
    text: '사진을 찍거나 본 마음을 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '초점이 조금 흐렸지만 마음에 드는 사진이에요.',
    hint: '사진을 올릴 필요 없이 상황만 상상하면 돼요.',
  },
  {
    key: 'everyday_photos_04',
    part: 'description',
    photo: false,
    starter: false,
    text: '사진을 찍거나 본 마음을 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '친구가 찍어 준 사진이 자연스러워서 좋아요.',
    hint: '사진을 올릴 필요 없이 상황만 상상하면 돼요.',
  },
  {
    key: 'everyday_photos_05',
    part: 'description',
    photo: false,
    starter: false,
    text: '사진을 찍거나 본 마음을 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '같은 장소를 다른 각도로 찍으니 느낌이 달라요.',
    hint: '사진을 올릴 필요 없이 상황만 상상하면 돼요.',
  },
  {
    key: 'everyday_photos_06',
    part: 'description',
    photo: false,
    starter: false,
    text: '사진을 찍거나 본 마음을 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '예전 사진을 보다가 그날의 기분이 떠올랐어요.',
    hint: '사진을 올릴 필요 없이 상황만 상상하면 돼요.',
  },
  {
    key: 'everyday_photos_07',
    part: 'description',
    photo: false,
    starter: false,
    text: '사진을 찍거나 본 마음을 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '음식을 찍으려다가 따뜻할 때 먹기로 했어요.',
    hint: '사진을 올릴 필요 없이 상황만 상상하면 돼요.',
  },
  {
    key: 'everyday_photos_08',
    part: 'description',
    photo: false,
    starter: false,
    text: '사진을 찍거나 본 마음을 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '잘 찍으려 애쓰지 않은 사진이 더 편하게 보여요.',
    hint: '사진을 올릴 필요 없이 상황만 상상하면 돼요.',
  },
  {
    key: 'everyday_small_tasks_01',
    part: 'description',
    photo: false,
    starter: false,
    text: '작은 일을 해낸 기분을 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '미뤄 둔 메일 한 통을 보내고 나니 속이 시원해요.',
    hint: '성공한 부분이나 아쉬운 부분 모두 좋아요.',
  },
  {
    key: 'everyday_small_tasks_02',
    part: 'description',
    photo: false,
    starter: false,
    text: '작은 일을 해낸 기분을 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '정리할 물건을 조금씩 나누니 덜 부담스러워요.',
    hint: '성공한 부분이나 아쉬운 부분 모두 좋아요.',
  },
  {
    key: 'everyday_small_tasks_03',
    part: 'description',
    photo: false,
    starter: false,
    text: '작은 일을 해낸 기분을 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '간단한 할 일을 적고 하나씩 지워 봤어요.',
    hint: '성공한 부분이나 아쉬운 부분 모두 좋아요.',
  },
  {
    key: 'everyday_small_tasks_04',
    part: 'description',
    photo: false,
    starter: false,
    text: '작은 일을 해낸 기분을 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '처음 해 보는 일을 설명을 보며 천천히 마쳤어요.',
    hint: '성공한 부분이나 아쉬운 부분 모두 좋아요.',
  },
  {
    key: 'everyday_small_tasks_05',
    part: 'description',
    photo: false,
    starter: false,
    text: '작은 일을 해낸 기분을 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '막히던 부분을 친구에게 물어보니 바로 해결됐어요.',
    hint: '성공한 부분이나 아쉬운 부분 모두 좋아요.',
  },
  {
    key: 'everyday_small_tasks_06',
    part: 'description',
    photo: false,
    starter: false,
    text: '작은 일을 해낸 기분을 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '완벽하게 하려다 시간이 오래 걸려 잠깐 멈췄어요.',
    hint: '성공한 부분이나 아쉬운 부분 모두 좋아요.',
  },
  {
    key: 'everyday_small_tasks_07',
    part: 'description',
    photo: false,
    starter: false,
    text: '작은 일을 해낸 기분을 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '오늘은 한 가지 일을 끝낸 것만으로 만족하려 해요.',
    hint: '성공한 부분이나 아쉬운 부분 모두 좋아요.',
  },
  {
    key: 'everyday_small_tasks_08',
    part: 'description',
    photo: false,
    starter: false,
    text: '작은 일을 해낸 기분을 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '내일의 나를 위해 필요한 물건을 미리 챙겼어요.',
    hint: '성공한 부분이나 아쉬운 부분 모두 좋아요.',
  },
  {
    key: 'everyday_neighborhood_01',
    part: 'description',
    photo: false,
    starter: false,
    text: '동네에서 발견한 점을 친구에게 소개해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '집 근처에 앉아서 쉴 벤치가 하나 있어요.',
    hint: '실제 장소 이름을 쓸 필요는 없어요.',
  },
  {
    key: 'everyday_neighborhood_02',
    part: 'description',
    photo: false,
    starter: false,
    text: '동네에서 발견한 점을 친구에게 소개해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '늘 지나던 골목에서 작은 꽃을 발견했어요.',
    hint: '실제 장소 이름을 쓸 필요는 없어요.',
  },
  {
    key: 'everyday_neighborhood_03',
    part: 'description',
    photo: false,
    starter: false,
    text: '동네에서 발견한 점을 친구에게 소개해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '새로 생긴 가게 앞을 지나며 한번 들어가 보고 싶어요.',
    hint: '실제 장소 이름을 쓸 필요는 없어요.',
  },
  {
    key: 'everyday_neighborhood_04',
    part: 'description',
    photo: false,
    starter: false,
    text: '동네에서 발견한 점을 친구에게 소개해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '가까운 공원이 생각보다 조용해요.',
    hint: '실제 장소 이름을 쓸 필요는 없어요.',
  },
  {
    key: 'everyday_neighborhood_05',
    part: 'description',
    photo: false,
    starter: false,
    text: '동네에서 발견한 점을 친구에게 소개해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '동네 길에 그늘이 많아서 걷기 편해요.',
    hint: '실제 장소 이름을 쓸 필요는 없어요.',
  },
  {
    key: 'everyday_neighborhood_06',
    part: 'description',
    photo: false,
    starter: false,
    text: '동네에서 발견한 점을 친구에게 소개해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '평소 문이 닫혀 있던 가게가 오늘은 열려 있어요.',
    hint: '실제 장소 이름을 쓸 필요는 없어요.',
  },
  {
    key: 'everyday_neighborhood_07',
    part: 'description',
    photo: false,
    starter: false,
    text: '동네에서 발견한 점을 친구에게 소개해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '집 근처에 늦게까지 하는 가게가 있어 든든해요.',
    hint: '실제 장소 이름을 쓸 필요는 없어요.',
  },
  {
    key: 'everyday_neighborhood_08',
    part: 'description',
    photo: false,
    starter: false,
    text: '동네에서 발견한 점을 친구에게 소개해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '동네에서 편하게 걸을 수 있는 길을 하나 찾았어요.',
    hint: '실제 장소 이름을 쓸 필요는 없어요.',
  },
  {
    key: 'everyday_feelings_01',
    part: 'description',
    photo: false,
    starter: false,
    text: '지금의 감정을 편하게 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '작은 일에도 웃음이 나는 날이에요.',
    hint: '감정 이름을 정확히 붙이지 않아도 괜찮아요.',
  },
  {
    key: 'everyday_feelings_02',
    part: 'description',
    photo: false,
    starter: false,
    text: '지금의 감정을 편하게 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '이유는 모르겠지만 오늘은 조금 느긋해요.',
    hint: '감정 이름을 정확히 붙이지 않아도 괜찮아요.',
  },
  {
    key: 'everyday_feelings_03',
    part: 'description',
    photo: false,
    starter: false,
    text: '지금의 감정을 편하게 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '기대했던 일이 취소돼서 아쉬워요.',
    hint: '감정 이름을 정확히 붙이지 않아도 괜찮아요.',
  },
  {
    key: 'everyday_feelings_04',
    part: 'description',
    photo: false,
    starter: false,
    text: '지금의 감정을 편하게 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '처음 해 보는 일을 앞두고 조금 긴장돼요.',
    hint: '감정 이름을 정확히 붙이지 않아도 괜찮아요.',
  },
  {
    key: 'everyday_feelings_05',
    part: 'description',
    photo: false,
    starter: false,
    text: '지금의 감정을 편하게 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '친구의 반가운 연락을 받고 기분이 좋아졌어요.',
    hint: '감정 이름을 정확히 붙이지 않아도 괜찮아요.',
  },
  {
    key: 'everyday_feelings_06',
    part: 'description',
    photo: false,
    starter: false,
    text: '지금의 감정을 편하게 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '할 일이 많아서 머릿속이 조금 복잡해요.',
    hint: '감정 이름을 정확히 붙이지 않아도 괜찮아요.',
  },
  {
    key: 'everyday_feelings_07',
    part: 'description',
    photo: false,
    starter: false,
    text: '지금의 감정을 편하게 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '일이 잘 풀려서 누구에게든 이야기하고 싶어요.',
    hint: '감정 이름을 정확히 붙이지 않아도 괜찮아요.',
  },
  {
    key: 'everyday_feelings_08',
    part: 'description',
    photo: false,
    starter: false,
    text: '지금의 감정을 편하게 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '혼자 조용히 있고 싶은 저녁이에요.',
    hint: '감정 이름을 정확히 붙이지 않아도 괜찮아요.',
  },
  {
    key: 'everyday_memories_01',
    part: 'description',
    photo: false,
    starter: false,
    text: '떠오른 순간을 친구에게 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '어떤 냄새를 맡으니 예전에 먹던 간식이 떠올라요.',
    hint: '실제 기억이 없다면 주어진 상황만 상상해도 좋아요.',
  },
  {
    key: 'everyday_memories_02',
    part: 'description',
    photo: false,
    starter: false,
    text: '떠오른 순간을 친구에게 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '익숙한 음악을 듣고 한동안 안 만난 친구가 생각나요.',
    hint: '실제 기억이 없다면 주어진 상황만 상상해도 좋아요.',
  },
  {
    key: 'everyday_memories_03',
    part: 'description',
    photo: false,
    starter: false,
    text: '떠오른 순간을 친구에게 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '오래된 메모를 보니 그때의 관심사가 새로워 보여요.',
    hint: '실제 기억이 없다면 주어진 상황만 상상해도 좋아요.',
  },
  {
    key: 'everyday_memories_04',
    part: 'description',
    photo: false,
    starter: false,
    text: '떠오른 순간을 친구에게 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '예전 사진 속 물건을 지금도 잘 쓰고 있어요.',
    hint: '실제 기억이 없다면 주어진 상황만 상상해도 좋아요.',
  },
  {
    key: 'everyday_memories_05',
    part: 'description',
    photo: false,
    starter: false,
    text: '떠오른 순간을 친구에게 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '전에 걷던 길을 다시 지나니 반가워요.',
    hint: '실제 기억이 없다면 주어진 상황만 상상해도 좋아요.',
  },
  {
    key: 'everyday_memories_06',
    part: 'description',
    photo: false,
    starter: false,
    text: '떠오른 순간을 친구에게 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '처음엔 낯설던 장소가 이제는 익숙하게 느껴져요.',
    hint: '실제 기억이 없다면 주어진 상황만 상상해도 좋아요.',
  },
  {
    key: 'everyday_memories_07',
    part: 'description',
    photo: false,
    starter: false,
    text: '떠오른 순간을 친구에게 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '친구와 나눴던 사소한 농담이 문득 떠올랐어요.',
    hint: '실제 기억이 없다면 주어진 상황만 상상해도 좋아요.',
  },
  {
    key: 'everyday_memories_08',
    part: 'description',
    photo: false,
    starter: false,
    text: '떠오른 순간을 친구에게 이야기해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '한때 좋아하던 음식을 오랜만에 다시 먹었어요.',
    hint: '실제 기억이 없다면 주어진 상황만 상상해도 좋아요.',
  },
  {
    key: 'everyday_recommendations_01',
    part: 'description',
    photo: false,
    starter: false,
    text: '친구에게 가볍게 제안하는 말을 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '친구가 너무 오래 앉아 있어서 잠깐 걷자고 하려 해요.',
    hint: '꼭 알아야 하는 정보나 지식은 없어도 돼요.',
  },
  {
    key: 'everyday_recommendations_02',
    part: 'description',
    photo: false,
    starter: false,
    text: '친구에게 가볍게 제안하는 말을 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '친구가 오늘 저녁은 간단히 먹고 싶대요.',
    hint: '꼭 알아야 하는 정보나 지식은 없어도 돼요.',
  },
  {
    key: 'everyday_recommendations_03',
    part: 'description',
    photo: false,
    starter: false,
    text: '친구에게 가볍게 제안하는 말을 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '친구가 물건을 사기 전에 고민하고 있어요.',
    hint: '꼭 알아야 하는 정보나 지식은 없어도 돼요.',
  },
  {
    key: 'everyday_recommendations_04',
    part: 'description',
    photo: false,
    starter: false,
    text: '친구에게 가볍게 제안하는 말을 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '친구가 쉬는 날에도 할 일을 잔뜩 적었어요.',
    hint: '꼭 알아야 하는 정보나 지식은 없어도 돼요.',
  },
  {
    key: 'everyday_recommendations_05',
    part: 'description',
    photo: false,
    starter: false,
    text: '친구에게 가볍게 제안하는 말을 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '친구가 새로운 취미를 가볍게 해 보고 싶대요.',
    hint: '꼭 알아야 하는 정보나 지식은 없어도 돼요.',
  },
  {
    key: 'everyday_recommendations_06',
    part: 'description',
    photo: false,
    starter: false,
    text: '친구에게 가볍게 제안하는 말을 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '친구가 집중이 안 돼서 자리를 옮길까 고민해요.',
    hint: '꼭 알아야 하는 정보나 지식은 없어도 돼요.',
  },
  {
    key: 'everyday_recommendations_07',
    part: 'description',
    photo: false,
    starter: false,
    text: '친구에게 가볍게 제안하는 말을 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '친구가 기분 전환할 작은 일을 찾고 있어요.',
    hint: '꼭 알아야 하는 정보나 지식은 없어도 돼요.',
  },
  {
    key: 'everyday_recommendations_08',
    part: 'description',
    photo: false,
    starter: false,
    text: '친구에게 가볍게 제안하는 말을 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '친구에게 요즘 마음에 드는 쉬는 방법을 알려주려 해요.',
    hint: '꼭 알아야 하는 정보나 지식은 없어도 돼요.',
  },
  {
    key: 'everyday_story_openings_01',
    part: 'opening',
    photo: false,
    starter: false,
    text: '이야기를 시작하는 첫마디를 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '오늘은 평소와 다르게 걸어서 나왔다는 이야기를 하려 해요.',
    hint: '친구에게 보내는 짧은 메시지처럼 시작해 주세요.',
  },
  {
    key: 'everyday_story_openings_02',
    part: 'opening',
    photo: false,
    starter: false,
    text: '이야기를 시작하는 첫마디를 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '우연히 맛있는 간식을 찾았다는 이야기를 꺼내려 해요.',
    hint: '친구에게 보내는 짧은 메시지처럼 시작해 주세요.',
  },
  {
    key: 'everyday_story_openings_03',
    part: 'opening',
    photo: false,
    starter: false,
    text: '이야기를 시작하는 첫마디를 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '별일 없던 하루에 작은 즐거움이 있었다고 말하려 해요.',
    hint: '친구에게 보내는 짧은 메시지처럼 시작해 주세요.',
  },
  {
    key: 'everyday_story_openings_04',
    part: 'opening',
    photo: false,
    starter: false,
    text: '이야기를 시작하는 첫마디를 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '비 오는 날 집에서 쉰 이야기를 시작하려 해요.',
    hint: '친구에게 보내는 짧은 메시지처럼 시작해 주세요.',
  },
  {
    key: 'everyday_story_openings_05',
    part: 'opening',
    photo: false,
    starter: false,
    text: '이야기를 시작하는 첫마디를 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '처음 써 본 작은 물건을 소개하려 해요.',
    hint: '친구에게 보내는 짧은 메시지처럼 시작해 주세요.',
  },
  {
    key: 'everyday_story_openings_06',
    part: 'opening',
    photo: false,
    starter: false,
    text: '이야기를 시작하는 첫마디를 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '새로 발견한 산책길 이야기를 꺼내려 해요.',
    hint: '친구에게 보내는 짧은 메시지처럼 시작해 주세요.',
  },
  {
    key: 'everyday_story_openings_07',
    part: 'opening',
    photo: false,
    starter: false,
    text: '이야기를 시작하는 첫마디를 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '조금 웃긴 실수를 했다는 이야기를 시작하려 해요.',
    hint: '친구에게 보내는 짧은 메시지처럼 시작해 주세요.',
  },
  {
    key: 'everyday_story_openings_08',
    part: 'opening',
    photo: false,
    starter: false,
    text: '이야기를 시작하는 첫마디를 써 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '갑자기 계획을 바꿨다는 이야기를 전하려 해요.',
    hint: '친구에게 보내는 짧은 메시지처럼 시작해 주세요.',
  },
  {
    key: 'everyday_story_closings_01',
    part: 'closing',
    photo: false,
    starter: false,
    text: '마지막 한마디로 이야기를 마무리해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '친구에게 새 간식을 추천하는 이야기를 다 했어요.',
    hint: '끝인사나 다음에 하고 싶은 일을 써도 좋아요.',
  },
  {
    key: 'everyday_story_closings_02',
    part: 'closing',
    photo: false,
    starter: false,
    text: '마지막 한마디로 이야기를 마무리해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '조용한 가게에서 쉬었던 이야기를 마무리하려 해요.',
    hint: '끝인사나 다음에 하고 싶은 일을 써도 좋아요.',
  },
  {
    key: 'everyday_story_closings_03',
    part: 'closing',
    photo: false,
    starter: false,
    text: '마지막 한마디로 이야기를 마무리해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '처음 써 본 물건의 좋고 아쉬운 점을 다 이야기했어요.',
    hint: '끝인사나 다음에 하고 싶은 일을 써도 좋아요.',
  },
  {
    key: 'everyday_story_closings_04',
    part: 'closing',
    photo: false,
    starter: false,
    text: '마지막 한마디로 이야기를 마무리해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '기다리는 동안 생긴 작은 일을 이야기했어요.',
    hint: '끝인사나 다음에 하고 싶은 일을 써도 좋아요.',
  },
  {
    key: 'everyday_story_closings_05',
    part: 'closing',
    photo: false,
    starter: false,
    text: '마지막 한마디로 이야기를 마무리해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '주말에 쉬기로 한 이유를 친구에게 설명했어요.',
    hint: '끝인사나 다음에 하고 싶은 일을 써도 좋아요.',
  },
  {
    key: 'everyday_story_closings_06',
    part: 'closing',
    photo: false,
    starter: false,
    text: '마지막 한마디로 이야기를 마무리해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '새로 찾은 산책길 이야기를 끝내려 해요.',
    hint: '끝인사나 다음에 하고 싶은 일을 써도 좋아요.',
  },
  {
    key: 'everyday_story_closings_07',
    part: 'closing',
    photo: false,
    starter: false,
    text: '마지막 한마디로 이야기를 마무리해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '계획을 바꾼 하루 이야기를 마무리하려 해요.',
    hint: '끝인사나 다음에 하고 싶은 일을 써도 좋아요.',
  },
  {
    key: 'everyday_story_closings_08',
    part: 'closing',
    photo: false,
    starter: false,
    text: '마지막 한마디로 이야기를 마무리해 보세요. 1~3문장으로 편하게 써 주세요.',
    scene: '친구에게 오늘 하루의 기분을 다 전했어요.',
    hint: '끝인사나 다음에 하고 싶은 일을 써도 좋아요.',
  },
] as const

export interface FakeVoicePrompt {
  key: string
  part: 'opening' | 'description' | 'closing'
  photo: boolean
  text: string
  scene?: string
  hint?: string
  starter?: boolean
}
export interface FakeVoiceRow {
  origin?: 'personal' | 'synthetic'
  id: string
  name: string
  isDefault?: boolean
  deleted?: boolean
  /** Omitted is made; a voice this fake creates starts not made, as on the server (VOICE-10). */
  made?: boolean
  /** How many 학습 글 the directory row says the voice holds. */
  materialCount?: number
}

/** The voice a fixture account holds unless a test lists its own directory: no account is given
 *  one (VOICE-4), so this stands for the one its owner made. The profile options below describe
 *  THIS voice's profile; every other voice starts empty, like the server. */
export const DEFAULT_FAKE_VOICE: FakeVoiceRow = {
  id: 'voice-default',
  name: '기본 말투',
  isDefault: true,
}

export interface FakeVoiceOptions {
  prompts?: readonly FakeVoicePrompt[]
  createFails?: boolean
  createGate?: Promise<void>

  activeJobId?: string
  /** The impression of an analysis the profile publishes on the read AFTER the first one — the
   *  shape a resumed analysis has when its job is already done. */
  analysisAfterAnalysis?: string
  /** The default voice's current analysis. Omitted, a made voice reads with an empty one. */
  analysis?: MessageInitShape<typeof VoiceAnalysisSchema>
  /** The default voice's previous analysis, which 이전 분석으로 되돌리기 returns to. */
  previousAnalysis?: MessageInitShape<typeof VoiceAnalysisSchema>
  /** The default voice's notice (VOICE-21). */
  notice?: { kind: 'added' | 'changed'; count?: number }
  samples?: FakeVoiceSampleRow[]
  addError?: string
  deleteFails?: boolean
  addGate?: Promise<void>
  updateGate?: Promise<void>
  updateRefusal?: AppFailureReason
  updateLosesFirstResponse?: boolean
  updates?: Array<{
    voiceId: string
    sampleId: string
    expectedContentRevision: bigint
    operationKey: string
    body?: string
    label?: string
    photoUploadId?: string
  }>
  analysisEstimate?: { free?: boolean; credits?: number }
  analysisEstimates?: Array<{ voiceId: string; model: string }>
  calls?: string[]
  /** The account's voice directory. Omitted, it holds only `DEFAULT_FAKE_VOICE`. */
  voices?: FakeVoiceRow[]
  /** Voice ids whose deletion the server refuses because work could still publish to them. */
  busyVoices?: string[]
  /** Make ListVoices fail. */
  listFails?: boolean
  /** Voice creates: a voice is created by name alone (VOICE-10). */
  creates?: Array<{ name: string }>
  /** 말투 만들기 / 다시 분석 starts, with the model each named. */
  analyses?: Array<{ voiceId: string; model: string }>
  /** Answers the fake received, with the upload each named. */
  answers?: Array<{ promptKey: string; body: string; uploadId: string }>
  /** The job an AnalyzeVoice answers with. */
  analyzeJobId?: string
  /** ②'s comparison per post slug (POST-102); a slug not listed answers not applicable. */
  postFingerprints?: Record<string, MessageInitShape<typeof GetPostFingerprintResponseSchema>>
  /** The slug of every GetPostFingerprint the fake received. */
  postFingerprintReads?: string[]
  /** The default voice's 검증 results, newest first (VOICE-43). */
  checks?: MessageInitShape<typeof VoiceCheckSchema>[]
  /** The default voice's queued or running 검증 job, as ListVoiceChecks names it. */
  activeCheckJobId?: string
  /** 검증 starts and retries the fake received, with the model each named. */
  checkStarts?: Array<{ voiceId: string; promptKey: string; model: string }>
  checkRetries?: Array<{ checkId: string; model: string }>
  /** Refuse a 검증 start with this reason — a shared entitlement refusal included. */
  checkStartRefusal?: { reason: AppFailureReason; code: Code; params?: Record<string, string> }
  /** The job a 검증 start or retry answers with. */
  checkJobId?: string
}

const PART_TO_PROTO = {
  opening: VoicePromptPart.OPENING,
  description: VoicePromptPart.DESCRIPTION,
  closing: VoicePromptPart.CLOSING,
} as const

/** The server's readiness, counted the simple way the fixtures need: sentences end at `.`, `!`,
 *  `?` or a line break, a post covers every part and an answer its prompt's (VOICE-32). */
function fixtureProse(body: string): string[] {
  return body
    .split(/\r?\n/)
    .filter(
      (line) =>
        !/^\s*(#|주소[: ]|영업시간[: ]|운영시간[: ]|전화[: ]|주차[: ]|위치[: ]|📍|⏰|☎️)/u.test(
          line,
        ),
    )
    .flatMap((line) => line.split(/[.!?]+/))
    .filter((part) => /[가-힣]/u.test(part))
}
function fakeReadiness(
  rows: readonly MaterialRow[],
  prompts: readonly FakeVoicePrompt[] = FAKE_VOICE_PROMPTS,
) {
  let sentences = 0
  const covered = new Set<string>(),
    answered = new Set<string>()
  for (const row of rows) {
    const prose = fixtureProse(row.body)
    sentences += prose.length
    if (prose.length === 0) continue
    if (row.sample.kind === VoiceSampleKind.ANSWER) {
      const prompt = prompts.find((candidate) => candidate.key === row.sample.promptKey)
      if (prompt) {
        covered.add(prompt.part)
        answered.add(prompt.key)
      }
    } else for (const part of ['opening', 'description', 'closing']) covered.add(part)
  }
  const missing = (['opening', 'description', 'closing'] as const).filter(
    (part) => !covered.has(part),
  )
  let percent = Math.max(
    Math.floor((Math.min(sentences, 60) * 100) / 60),
    Math.floor((Math.min(answered.size, 10) * 100) / 10),
  )
  if (missing.length > 0 && percent > 99) percent = 99
  return create(VoiceReadinessSchema, {
    percent,
    sentences,
    needed: 60,
    answeredQuestions: answered.size,
    requiredQuestions: 10,
    missingParts: missing.map((part) => PART_TO_PROTO[part]),
  })
}

interface MaterialRow {
  sample: ProtoVoiceSample
  body: string
}

const NOW = '2026-08-29T12:00:00Z'
const NAME_MAX_CHARS = 50

interface VoiceRow {
  id: string
  name: string
  isDefault: boolean
  deletedAt: string
  made: boolean
  materialCount: number
}

const toSampleKind = (kind: FakeVoiceSampleRow['kind']) =>
  kind === 'answer' ? VoiceSampleKind.ANSWER : VoiceSampleKind.POST

export function registerVoiceService(router: ConnectRouter, options: FakeVoiceOptions = {}) {
  const promptBank = options.prompts ?? FAKE_VOICE_PROMPTS
  const { rpc } = router
  let sequence = options.samples?.length ?? 0
  let profileReads = 0
  let hasAnalysisAdmission = !!options.activeJobId
  let acceptedAtAdmission: Array<{ sampleId: string; contentRevision: bigint }> = []
  let initialNotice = options.notice

  const voices = new Map<string, VoiceRow>(
    (options.voices ?? [DEFAULT_FAKE_VOICE]).map((row) => [
      row.id,
      {
        id: row.id,
        name: row.name,
        isDefault: row.isDefault ?? false,
        deletedAt: row.deleted ? NOW : '',
        made: row.made ?? true,
        materialCount: row.materialCount ?? 0,
      },
    ]),
  )
  let voiceSequence = voices.size
  const defaultId =
    [...voices.values()].find((row) => row.isDefault && !row.deletedAt)?.id ?? DEFAULT_FAKE_VOICE.id

  // Every voice's 학습 글, newest first. The options' samples are the default voice's.
  const materials = new Map<string, MaterialRow[]>()
  const materialsOf = (voiceId: string) => materials.get(voiceId) ?? []
  const toProtoVoice = (row: VoiceRow) =>
    create(VoiceSchema, {
      id: row.id,
      name: row.name,
      isDefault: row.isDefault,
      deleted: row.deletedAt !== '',
      createdAt: NOW,
      updatedAt: NOW,
      deletedAt: row.deletedAt,
      made: row.made,
      materialCount: materials.has(row.id) ? materialsOf(row.id).length : row.materialCount,
      analyzedAt: row.made ? NOW : '',
      readinessPercent: row.made ? 0 : fakeReadiness(materialsOf(row.id), promptBank).percent,
    })
  const compare = (a: string, b: string) => (a < b ? -1 : a > b ? 1 : 0)
  // The server's order: active before deleted, the default first, then by name.
  const directory = () =>
    [...voices.values()].sort(
      (a, b) =>
        Number(a.deletedAt !== '') - Number(b.deletedAt !== '') ||
        Number(b.isDefault) - Number(a.isDefault) ||
        compare(a.name, b.name) ||
        compare(a.id, b.id),
    )
  const owned = (voiceId: string): VoiceRow => {
    if (!voiceId) throw connectAppError('VOICE_REQUIRED', Code.InvalidArgument)
    const row = voices.get(voiceId)
    if (!row) throw connectAppError('VOICE_NOT_FOUND', Code.NotFound)
    return row
  }
  const active = (voiceId: string): VoiceRow => {
    const row = owned(voiceId)
    if (row.deletedAt) throw connectAppError('VOICE_DELETED', Code.FailedPrecondition)
    return row
  }
  const validName = (name: string): string => {
    const trimmed = name.trim()
    const chars = Array.from(trimmed).length
    if (chars === 0) throw connectAppError('VOICE_NAME_REQUIRED', Code.InvalidArgument)
    if (chars > NAME_MAX_CHARS)
      throw connectAppError('VOICE_NAME_TOO_LONG', Code.InvalidArgument, {
        actual: String(chars),
        max: String(NAME_MAX_CHARS),
      })
    return trimmed
  }
  const nameTaken = (name: string, except: string) =>
    [...voices.values()].some((row) => row.id !== except && !row.deletedAt && row.name === name)

  // Profiles are partitioned per voice: the options describe the default voice's, and any other
  // voice — created here or listed in `voices` — starts empty, as on the server.
  const profiles = new Map<string, ProtoVoiceProfile>()
  if (options.samples) {
    materials.set(
      defaultId,
      options.samples.map((row) => ({
        sample: create(VoiceSampleSchema, {
          id: row.id,
          label: row.label,
          chars: row.chars ?? 200,
          createdAt: row.createdAt ?? NOW,
          kind: toSampleKind(row.kind),
          promptKey: row.promptKey ?? '',
          hasPhoto: row.hasPhoto ?? false,
          contentRevision: row.contentRevision ?? 1n,
        }),
        body: row.body ?? '가'.repeat(row.chars ?? 200),
      })),
    )
  }
  profiles.set(
    defaultId,
    create(VoiceProfileSchema, {
      activeJobId: options.activeJobId ?? '',
    }),
  )
  // Each voice's current and previous analysis (VOICE-30). A made voice with no analysis given
  // reads with an empty one, so it is shown as made.
  const current = new Map<string, ReturnType<typeof create<typeof VoiceAnalysisSchema>>>()
  const previous = new Map<string, ReturnType<typeof create<typeof VoiceAnalysisSchema>>>()
  if (options.analysis) current.set(defaultId, create(VoiceAnalysisSchema, options.analysis))
  if (options.previousAnalysis)
    previous.set(defaultId, create(VoiceAnalysisSchema, options.previousAnalysis))
  // Like the server, an example whose 학습 글 was deleted is not returned (VOICE-21).
  const withoutDeletedExamples = (voiceId: string, analysis: ProtoVoiceAnalysis) => {
    const present = new Set(materialsOf(voiceId).map((row) => row.sample.id))
    const copy = create(VoiceAnalysisSchema, analysis)
    const counted = copy.counted
    if (counted) {
      for (const item of [
        counted.endings,
        counted.marks,
        counted.emoji,
        counted.shape,
        counted.openings,
        counted.adverbs,
        counted.person,
        counted.headings,
      ]) {
        if (item?.example && !present.has(item.example.materialId)) item.example = undefined
      }
    }
    if (copy.ai)
      copy.ai.examples = copy.ai.examples.filter((example) => present.has(example.materialId))
    return copy
  }
  const analysisOf = (row: VoiceRow) => {
    const found = current.get(row.id)
    if (found) return withoutDeletedExamples(row.id, found)
    return row.made ? create(VoiceAnalysisSchema, { counted: {}, ai: {} }) : undefined
  }
  const profileOf = (voiceId: string): ProtoVoiceProfile => {
    owned(voiceId)
    let profile = profiles.get(voiceId)
    if (!profile) {
      profile = create(VoiceProfileSchema, {})
      profiles.set(voiceId, profile)
    }
    return profile
  }
  const setProfile = (voiceId: string, profile: ProtoVoiceProfile) => profiles.set(voiceId, profile)
  const withVoice = (voiceId: string, profile: ProtoVoiceProfile) => {
    const row = owned(voiceId)
    const analysis = analysisOf(row)
    const currentVersions = new Map(
      materialsOf(voiceId).map((material) => [material.sample.id, material.sample.contentRevision]),
    )
    const versionChanges =
      analysis?.sourceVersionsKnown &&
      (analysis.acceptedSources.length !== currentVersions.size ||
        analysis.acceptedSources.some(
          (source) => currentVersions.get(source.sampleId) !== source.contentRevision,
        ))
    const notice =
      voiceId === defaultId && initialNotice
        ? create(VoiceNoticeSchema, {
            kind: initialNotice.kind === 'added' ? VoiceNoticeKind.ADDED : VoiceNoticeKind.CHANGED,
            count: initialNotice.count ?? 0,
          })
        : versionChanges
          ? create(VoiceNoticeSchema, { kind: VoiceNoticeKind.CHANGED })
          : undefined
    return create(VoiceProfileSchema, {
      ...profile,
      voice: toProtoVoice(row),
      made: row.made,
      samples: materialsOf(voiceId).map((material) => material.sample),
      readiness: fakeReadiness(materialsOf(voiceId), promptBank),
      analysis: analysisOf(row),
      hasPrevious: previous.has(voiceId),
      notice,
    })
  }

  rpc(VoiceService.method.listVoices, () => {
    options.calls?.push('ListVoices')
    if (options.listFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    return create(ListVoicesResponseSchema, { voices: directory().map(toProtoVoice) })
  })

  rpc(VoiceService.method.createVoice, async (request) => {
    options.calls?.push('CreateVoice')
    if (options.createGate) await options.createGate
    if (options.createFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    const name = validName(request.name)
    if (nameTaken(name, '')) throw connectAppError('VOICE_NAME_TAKEN', Code.AlreadyExists)
    options.creates?.push({ name })
    voiceSequence += 1
    const row: VoiceRow = {
      id: `voice-${voiceSequence}`,
      name,
      isDefault: false,
      deletedAt: '',
      made: false,
      materialCount: 0,
    }
    voices.set(row.id, row)
    return create(CreateVoiceResponseSchema, { voice: toProtoVoice(row) })
  })

  rpc(VoiceService.method.renameVoice, (request) => {
    options.calls?.push('RenameVoice')
    const row = owned(request.voiceId)
    const name = validName(request.name)
    if (!row.deletedAt && nameTaken(name, row.id)) {
      throw connectAppError('VOICE_NAME_TAKEN', Code.AlreadyExists)
    }
    row.name = name
    return create(RenameVoiceResponseSchema, { voice: toProtoVoice(row) })
  })

  rpc(VoiceService.method.setDefaultVoice, (request) => {
    options.calls?.push('SetDefaultVoice')
    // An empty id clears the 기본 (VOICE-12); a voice not yet made cannot be it.
    if (!request.voiceId) {
      for (const other of voices.values()) other.isDefault = false
      return create(SetDefaultVoiceResponseSchema, { voices: directory().map(toProtoVoice) })
    }
    const row = active(request.voiceId)
    if (!row.made) throw connectAppError('VOICE_NOT_MADE', Code.FailedPrecondition)
    for (const other of voices.values()) other.isDefault = false
    row.isDefault = true
    return create(SetDefaultVoiceResponseSchema, { voices: directory().map(toProtoVoice) })
  })

  rpc(VoiceService.method.deleteVoice, (request) => {
    options.calls?.push('DeleteVoice')
    const row = owned(request.voiceId)
    // The 기본 and the last voice delete like any other (VOICE-13).
    if (!row.deletedAt) {
      if (options.busyVoices?.includes(row.id)) {
        throw connectAppError('VOICE_BUSY', Code.FailedPrecondition)
      }
      row.deletedAt = NOW
      row.isDefault = false
    }
    return create(DeleteVoiceResponseSchema, { voice: toProtoVoice(row) })
  })

  rpc(VoiceService.method.restoreVoice, (request) => {
    options.calls?.push('RestoreVoice')
    const row = owned(request.voiceId)
    if (row.deletedAt) {
      if (nameTaken(row.name, row.id)) {
        throw connectAppError('VOICE_NAME_TAKEN', Code.AlreadyExists)
      }
      row.deletedAt = ''
    }
    return create(RestoreVoiceResponseSchema, { voice: toProtoVoice(row) })
  })

  rpc(VoiceService.method.getVoiceProfile, (request) => {
    options.calls?.push('GetVoiceProfile')
    let profile = profileOf(request.voiceId)
    if (request.voiceId === defaultId) {
      if (profileReads > 0 && options.analysisAfterAnalysis && hasAnalysisAdmission) {
        profile = create(VoiceProfileSchema, { ...profile, activeJobId: '' })
        setProfile(defaultId, profile)
        const prior = current.get(defaultId)
        if (prior) previous.set(defaultId, prior)
        current.set(
          defaultId,
          create(VoiceAnalysisSchema, {
            counted: {},
            ai: { impression: options.analysisAfterAnalysis },
            sourceVersionsKnown: true,
            acceptedSources: acceptedAtAdmission,
          }),
        )
        owned(defaultId).made = true
        hasAnalysisAdmission = false
        initialNotice = undefined
      }
      profileReads += 1
    }
    return create(GetVoiceProfileResponseSchema, { profile: withVoice(request.voiceId, profile) })
  })

  // 검증 (VOICE-43): the default voice's results, a start or retry adding a queued check.
  const checks = new Map<string, ProtoVoiceCheck[]>()
  checks.set(
    defaultId,
    (options.checks ?? []).map((check) => create(VoiceCheckSchema, check)),
  )
  let checkSequence = 0
  const queuedCheck = (voiceId: string, promptKey: string) => {
    const prompt = promptBank.find((candidate) => candidate.key === promptKey)
    if (!prompt) throw connectAppError('VOICE_PROMPT_NOT_FOUND', Code.NotFound)
    const answer = materialsOf(voiceId).find((row) => row.sample.promptKey === promptKey)
    if (!answer) throw connectAppError('VOICE_CHECK_PROMPT_UNANSWERED', Code.FailedPrecondition)
    checkSequence += 1
    const check = create(VoiceCheckSchema, {
      id: `check-new-${checkSequence}`,
      prompt: {
        key: prompt.key,
        part: PART_TO_PROTO[prompt.part],
        photo: prompt.photo,
        text: prompt.text,
      },
      answer: answer.body,
      status: ProtoVoiceCheckStatus.QUEUED,
      createdAt: NOW,
    })
    checks.set(voiceId, [check, ...(checks.get(voiceId) ?? [])])
    return check
  }
  rpc(VoiceService.method.listVoiceChecks, (request) => {
    options.calls?.push('ListVoiceChecks')
    owned(request.voiceId)
    return create(ListVoiceChecksResponseSchema, {
      checks: checks.get(request.voiceId) ?? [],
      activeJobId: request.voiceId === defaultId ? (options.activeCheckJobId ?? '') : '',
    })
  })
  rpc(VoiceService.method.startVoiceCheck, (request) => {
    options.calls?.push('StartVoiceCheck')
    const model = `${request.model?.providerId ?? ''}/${request.model?.modelId ?? ''}`
    options.checkStarts?.push({ voiceId: request.voiceId, promptKey: request.promptKey, model })
    active(request.voiceId)
    if (options.checkStartRefusal) {
      const { reason, code, params } = options.checkStartRefusal
      throw connectAppError(reason, code, params)
    }
    const check = queuedCheck(request.voiceId, request.promptKey)
    return create(StartVoiceCheckResponseSchema, {
      check,
      jobId: options.checkJobId ?? 'check-job',
    })
  })
  rpc(VoiceService.method.retryVoiceCheck, (request) => {
    options.calls?.push('RetryVoiceCheck')
    const model = `${request.model?.providerId ?? ''}/${request.model?.modelId ?? ''}`
    options.checkRetries?.push({ checkId: request.checkId, model })
    for (const [voiceId, rows] of checks) {
      const found = rows.find((row) => row.id === request.checkId)
      if (found?.prompt) {
        const check = queuedCheck(voiceId, found.prompt.key)
        return create(RetryVoiceCheckResponseSchema, {
          check,
          jobId: options.checkJobId ?? 'check-job',
        })
      }
    }
    throw connectAppError('VOICE_CHECK_NOT_FOUND', Code.NotFound)
  })

  rpc(VoiceService.method.getPostFingerprint, (request) => {
    options.calls?.push('GetPostFingerprint')
    options.postFingerprintReads?.push(request.postSlug)
    return create(
      GetPostFingerprintResponseSchema,
      options.postFingerprints?.[request.postSlug] ?? { applicable: false },
    )
  })

  rpc(VoiceService.method.restorePreviousVoiceAnalysis, (request) => {
    options.calls?.push('RestorePreviousVoiceAnalysis')
    active(request.voiceId)
    const back = previous.get(request.voiceId)
    if (!back) throw connectAppError('VOICE_NO_PREVIOUS_ANALYSIS', Code.FailedPrecondition)
    current.set(request.voiceId, back)
    previous.delete(request.voiceId)
    return create(RestorePreviousVoiceAnalysisResponseSchema, {
      profile: withVoice(request.voiceId, profileOf(request.voiceId)),
    })
  })

  const addMaterial = (voiceId: string, material: MaterialRow) =>
    materials.set(voiceId, [material, ...materialsOf(voiceId)])

  rpc(VoiceService.method.addVoiceSample, async (request) => {
    options.calls?.push('AddVoiceSample')
    active(request.voiceId)
    if (options.addGate) await options.addGate
    if (options.addError)
      throw connectAppError('VOICE_SAMPLE_TOO_SHORT', Code.InvalidArgument, {
        actual: '199',
        min: '200',
      })
    const body = request.body.trim()
    const chars = Array.from(body).length
    if (chars < 200) {
      throw connectAppError('VOICE_SAMPLE_TOO_SHORT', Code.InvalidArgument, {
        actual: String(chars),
        min: '200',
      })
    }
    sequence += 1
    const sample = create(VoiceSampleSchema, {
      id: `sample-${sequence}`,
      kind: VoiceSampleKind.POST,
      contentRevision: 1n,
      label: request.label.trim() || body.slice(0, 20),
      chars,
      createdAt: NOW,
    })
    addMaterial(request.voiceId, { sample, body })
    return create(AddVoiceSampleResponseSchema, { sample })
  })

  rpc(VoiceService.method.deleteVoiceSample, (request) => {
    options.calls?.push('DeleteVoiceSample')
    active(request.voiceId)
    if (options.deleteFails) throw connectAppError('UNKNOWN_FAILURE', Code.Internal)
    const rows = materialsOf(request.voiceId)
    const kept = rows.filter((row) => row.sample.id !== request.sampleId)
    if (kept.length === rows.length) {
      throw connectAppError('VOICE_SAMPLE_NOT_FOUND', Code.NotFound)
    }
    materials.set(request.voiceId, kept)
    return create(DeleteVoiceSampleResponseSchema, {})
  })

  rpc(VoiceService.method.getVoiceSample, (request) => {
    options.calls?.push('GetVoiceSample')
    owned(request.voiceId)
    const row = materialsOf(request.voiceId).find(
      (material) => material.sample.id === request.sampleId,
    )
    if (!row) throw connectAppError('VOICE_SAMPLE_NOT_FOUND', Code.NotFound)
    return create(GetVoiceSampleResponseSchema, {
      sample: row.sample,
      body: row.body,
      photoUrl: row.sample.hasPhoto ? `https://storage.test/voices/${request.sampleId}.jpg` : '',
      photoWidth: row.sample.hasPhoto ? 1024 : 0,
      photoHeight: row.sample.hasPhoto ? 768 : 0,
    })
  })

  rpc(VoiceService.method.listVoicePrompts, () => {
    options.calls?.push('ListVoicePrompts')
    return create(ListVoicePromptsResponseSchema, {
      prompts: promptBank.map((prompt) =>
        create(VoicePromptSchema, { ...prompt, part: PART_TO_PROTO[prompt.part] }),
      ),
    })
  })

  let uploadSequence = 0
  const pendingUploads = new Map<string, string>()
  rpc(VoiceService.method.createVoicePhotoUpload, (request) => {
    options.calls?.push('CreateVoicePhotoUpload')
    active(request.voiceId)
    const prompt = promptBank.find((candidate) => candidate.key === request.promptKey)
    if (!prompt?.photo) throw connectAppError('VOICE_PROMPT_NOT_FOUND', Code.NotFound)
    uploadSequence += 1
    const uploadId = `voice-upload-${uploadSequence}`
    pendingUploads.set(uploadId, request.promptKey)
    return create(CreateVoicePhotoUploadResponseSchema, {
      uploadId,
      putUrl: `https://storage.test/put/${uploadId}`,
      contentType: 'image/jpeg',
      expiresAt: NOW,
    })
  })

  rpc(VoiceService.method.answerVoicePrompt, (request) => {
    options.calls?.push('AnswerVoicePrompt')
    active(request.voiceId)
    const prompt = promptBank.find((candidate) => candidate.key === request.promptKey)
    if (!prompt) throw connectAppError('VOICE_PROMPT_NOT_FOUND', Code.NotFound)
    const body = request.body.trim()
    if (!body) throw connectAppError('VOICE_ANSWER_REQUIRED', Code.InvalidArgument)
    // A second answer rewrites the first, on its photo unless another is uploaded (VOICE-60).
    const rows = materialsOf(request.voiceId)
    const previous = rows.find((row) => row.sample.promptKey === prompt.key)
    const keepsPhoto = !request.uploadId && previous?.sample.hasPhoto === true
    if (prompt.photo && !keepsPhoto && pendingUploads.get(request.uploadId) !== prompt.key) {
      throw connectAppError('VOICE_PHOTO_REQUIRED', Code.FailedPrecondition)
    }
    if (previous)
      materials.set(
        request.voiceId,
        rows.filter((row) => row !== previous),
      )
    options.answers?.push({ promptKey: prompt.key, body, uploadId: request.uploadId })
    pendingUploads.delete(request.uploadId)
    sequence += 1
    const sample = create(VoiceSampleSchema, {
      id: `sample-${sequence}`,
      kind: VoiceSampleKind.ANSWER,
      contentRevision: (previous?.sample.contentRevision ?? 0n) + 1n,
      promptKey: prompt.key,
      hasPhoto: prompt.photo,
      chars: Array.from(body).length,
      createdAt: NOW,
    })
    addMaterial(request.voiceId, { sample, body })
    return create(AnswerVoicePromptResponseSchema, { sample })
  })

  const updateReceipts = new Map<string, { signature: string; sample: ProtoVoiceSample }>()
  let updateLost = false
  rpc(VoiceService.method.updateVoiceSample, async (request) => {
    options.calls?.push('UpdateVoiceSample')
    active(request.voiceId)
    options.updates?.push({
      voiceId: request.voiceId,
      sampleId: request.sampleId,
      expectedContentRevision: request.expectedContentRevision,
      operationKey: request.operationKey,
      body: request.body,
      label: request.label,
      photoUploadId: request.photoUploadId,
    })
    const signature = JSON.stringify([
      request.sampleId,
      String(request.expectedContentRevision),
      request.label,
      request.body,
      request.photoUploadId,
      request.photoWidth,
      request.photoHeight,
    ])
    const key = JSON.stringify([request.voiceId, request.sampleId, request.operationKey])
    const oldReceipt = updateReceipts.get(key)
    if (oldReceipt) {
      if (oldReceipt.signature !== signature)
        throw connectAppError('VOICE_SAMPLE_UPDATE_INVALID', Code.InvalidArgument)
      return create(UpdateVoiceSampleResponseSchema, { sample: oldReceipt.sample })
    }
    if (options.updateGate) await options.updateGate
    if (options.updateRefusal) throw connectAppError(options.updateRefusal, Code.Aborted)
    const row = materialsOf(request.voiceId).find((item) => item.sample.id === request.sampleId)
    if (!row) throw connectAppError('VOICE_SAMPLE_NOT_FOUND', Code.NotFound)
    if (request.expectedContentRevision !== row.sample.contentRevision)
      throw connectAppError('VOICE_SAMPLE_REVISION_CONFLICT', Code.Aborted)
    if (!request.operationKey)
      throw connectAppError('VOICE_SAMPLE_UPDATE_INVALID', Code.InvalidArgument)
    const body = request.body === undefined ? row.body : request.body.trim()
    const chars = Array.from(body).length
    if (row.sample.kind === VoiceSampleKind.POST && chars < 200)
      throw connectAppError('VOICE_SAMPLE_TOO_SHORT', Code.InvalidArgument, {
        actual: String(chars),
        min: '200',
      })
    if (row.sample.kind === VoiceSampleKind.ANSWER && !body)
      throw connectAppError('VOICE_ANSWER_REQUIRED', Code.InvalidArgument)
    const prompt = promptBank.find((item) => item.key === row.sample.promptKey)
    let hasPhoto = row.sample.hasPhoto
    if (request.photoUploadId !== undefined) {
      if (!request.photoUploadId) hasPhoto = false
      else {
        if (
          !prompt?.photo ||
          pendingUploads.get(request.photoUploadId) !== row.sample.promptKey ||
          (request.photoWidth ?? 0) <= 0 ||
          (request.photoHeight ?? 0) <= 0
        )
          throw connectAppError('VOICE_SAMPLE_UPDATE_INVALID', Code.InvalidArgument)
        hasPhoto = true
      }
    }
    if (prompt?.photo && !hasPhoto)
      throw connectAppError('VOICE_PHOTO_REQUIRED', Code.FailedPrecondition)
    const semanticChange =
      body !== row.body || hasPhoto !== row.sample.hasPhoto || !!request.photoUploadId
    row.body = body
    if (request.label !== undefined)
      row.sample.label = request.label.trim() || Array.from(body).slice(0, 20).join('')
    row.sample.chars = chars
    row.sample.hasPhoto = hasPhoto
    if (semanticChange) {
      row.sample.contentRevision++
      if (request.voiceId === defaultId) initialNotice = { kind: 'changed' }
    }
    if (request.photoUploadId) pendingUploads.delete(request.photoUploadId)
    const receipt = create(VoiceSampleSchema, row.sample)
    updateReceipts.set(key, { signature, sample: receipt })
    if (options.updateLosesFirstResponse && !updateLost) {
      updateLost = true
      throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    }
    return create(UpdateVoiceSampleResponseSchema, { sample: receipt })
  })
  rpc(VoiceService.method.estimateVoiceAnalysis, (request) => {
    options.calls?.push('EstimateVoiceAnalysis')
    active(request.voiceId)
    if (!request.model?.providerId || !request.model.modelId)
      throw connectAppError('VOICE_ANALYZE_MODEL_REQUIRED', Code.FailedPrecondition)
    if (fakeReadiness(materialsOf(request.voiceId), promptBank).percent < 100)
      throw connectAppError('VOICE_NOT_READY', Code.FailedPrecondition)
    options.analysisEstimates?.push({
      voiceId: request.voiceId,
      model: `${request.model.providerId}/${request.model.modelId}`,
    })
    return create(VoiceService.method.estimateVoiceAnalysis.output, {
      free: options.analysisEstimate?.free ?? false,
      credits: options.analysisEstimate?.credits ?? 3,
    })
  })
  rpc(VoiceService.method.analyzeVoice, (request) => {
    options.calls?.push('AnalyzeVoice')
    active(request.voiceId)
    if (!request.model?.providerId || !request.model.modelId) {
      throw connectAppError('VOICE_ANALYZE_MODEL_REQUIRED', Code.FailedPrecondition)
    }
    if (fakeReadiness(materialsOf(request.voiceId), promptBank).percent < 100) {
      throw connectAppError('VOICE_NOT_READY', Code.FailedPrecondition)
    }
    hasAnalysisAdmission = true
    acceptedAtAdmission = materialsOf(request.voiceId).map((material) => ({
      sampleId: material.sample.id,
      contentRevision: material.sample.contentRevision,
    }))
    options.analyses?.push({
      voiceId: request.voiceId,
      model: `${request.model.providerId}/${request.model.modelId}`,
    })
    const jobId = options.analyzeJobId ?? 'voice-job'
    setProfile(
      request.voiceId,
      create(VoiceProfileSchema, { ...profileOf(request.voiceId), activeJobId: jobId }),
    )
    return create(AnalyzeVoiceResponseSchema, { jobId })
  })
}
