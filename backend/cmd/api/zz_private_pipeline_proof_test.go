package main

import (
 "testing"
 "crypto/sha256"
 "encoding/hex"
 "sync"
 "errors"
 "github.com/postpilot/backend/internal/clip/diagnostic/analysisquality"
 "context"
 "os"
 "path/filepath"
 "encoding/json"
 "time"
 "strings"
 "net/http"
 "io"
 "math/big"
 "fmt"
 "github.com/postpilot/backend/internal/llm"
 "github.com/postpilot/backend/internal/clip"
 "github.com/postpilot/backend/internal/clip/ai"
)

type privateProofSource struct{ values []llm.SourceModel }
func(s privateProofSource)Models()[]llm.SourceModel{return s.values}
func(s privateProofSource)Lookup(id string)(llm.SourceModel,bool){for _,m:=range s.values{if m.ModelID==id{return m,true}};return llm.SourceModel{},false}
func proofRead(t *testing.T,p string,v any){t.Helper();b,e:=os.ReadFile(p);if e!=nil{t.Fatal(e)};if e=json.Unmarshal(b,v);e!=nil{t.Fatal(e)}}
func proofPrice(s string)string{v,ok:=new(big.Rat).SetString(s);if !ok{return ""};return v.Mul(v,big.NewRat(1000000,1)).FloatString(12)}
func proofRegistry(t *testing.T)(string,*llm.Registry){
 t.Helper();root:=os.Getenv("POSTPILOT_PRIVATE_PROOF_ROOT");if root==""{t.Skip("explicit private operator proof only")};root,e:=filepath.EvalSymlinks(root);if e!=nil{t.Fatal(e)}
 if os.Getenv("POSTPILOT_APPROVED_LIVE_CAP_MICROUSD")!="200000"{t.Fatal("explicit owner cap not supplied")}
 var approval map[string]any;proofRead(t,filepath.Join(root,"spend-and-push-approval.json"),&approval);if approval["authorizedOpenRouterCumulativeUSD"]!="0.20"{t.Fatal("approval mismatch")}
 var admitted struct{Key string `json:"OPENROUTER_API_KEY"`};proofRead(t,filepath.Join(root,"live-admission.private.json"),&admitted);if admitted.Key==""{t.Fatal("missing existing provider key")}
 var rows []struct{Model map[string]any `json:"model"`;Purposes []struct{Purpose,Level string;ReasoningEffort *string `json:"reasoning_effort"`} `json:"purposes"`};proofRead(t,filepath.Join(root,"live-catalog.private.json"),&rows)
 request,e:=http.NewRequestWithContext(context.Background(),"GET","https://openrouter.ai/api/v1/models",nil);if e!=nil{t.Fatal(e)};res,e:=(&http.Client{Timeout:30*time.Second}).Do(request);if e!=nil{t.Fatal(e)};defer res.Body.Close();if res.StatusCode!=200{t.Fatal("model metadata unavailable")}
 var live struct{Data []struct{ID string;ContextLength int64 `json:"context_length"`;Pricing struct{Prompt,Completion string};Architecture struct{Input []string `json:"input_modalities"`}}};if e=json.NewDecoder(io.LimitReader(res.Body,32<<20)).Decode(&live);e!=nil{t.Fatal(e)}
 source:=privateProofSource{}
 for _,row:=range rows{m:=row.Model;id,_:=m["model_id"].(string);var current *struct{ID string;ContextLength int64 `json:"context_length"`;Pricing struct{Prompt,Completion string};Architecture struct{Input []string `json:"input_modalities"`}};for i:=range live.Data{if live.Data[i].ID==id{current=&live.Data[i];break}};if current==nil{continue}
  s:=llm.SourceModel{ModelID:id,Label:id,Vision:true,VideoInput:true,StructuredOutput:m["structured_output"]==float64(1),ContextTokens:current.ContextLength,InputUSDPerMillion:proofPrice(current.Pricing.Prompt),OutputUSDPerMillion:proofPrice(current.Pricing.Completion),PricingCheckedAt:time.Now().UTC().Format("2006-01-02"),ReasoningNativeEffort:m["reasoning_native_effort"]==float64(1),Reasoning:map[string]llm.ReasoningEffort{},Levels:map[string]string{}}
  if raw,ok:=m["reasoning_efforts"].(string);ok&&raw!=""{if json.Unmarshal([]byte(raw),&s.ReasoningEfforts)!=nil{for _,v:=range strings.Split(raw,","){s.ReasoningEfforts=append(s.ReasoningEfforts,llm.ReasoningEffort(v))}}}
  for _,p:=range row.Purposes{stage:="";switch p.Purpose{case "photo-analysis":stage=llm.StageNameObserve;case "style-analysis":stage=llm.StageNameAnalyze;case "writing":stage=llm.StageNameWrite};if stage!=""{s.Stages=append(s.Stages,stage);s.Levels[stage]=p.Level;if p.ReasoningEffort!=nil{s.Reasoning[stage]=llm.ReasoningEffort(*p.ReasoningEffort)}}};source.values=append(source.values,s)
 }
 connection:=[]byte("providers:\n  - id: openrouter\n    adapter: openai_compatible\n    base_url: https://openrouter.ai/api/v1\n    api_key_env: OPENROUTER_API_KEY\n    reasoning_format: openrouter\n")
 registry,e:=llm.Parse(connection,func(name string)string{if name=="OPENROUTER_API_KEY"{return admitted.Key};return ""},adapters,source,llm.Options{Timeout:5*time.Minute,MaxTokens:32768,EndpointFetchTimeout:30*time.Second,EndpointCacheTTL:5*time.Minute});if e!=nil{t.Fatal(e)};return root,registry
}
func TestPrivateLiveClipPreflight(t *testing.T){root,r:=proofRegistry(t);cfg:=ai.DefaultConfig(clip.Environment{});var policies []llm.CallPolicy
 for _,v:=range []struct{model,stage string;tokens int;delivery llm.ExecutionDelivery}{{"google/gemini-3.8-flash",llm.StageNameObserve,cfg.ObserveCompletionTokens,llm.ExecutionInlineStatic},{"qwen/qwen3.8-flash",llm.StageNameObserve,cfg.ObserveCompletionTokens,llm.ExecutionInlineStatic},{"qwen/qwen3.8-flash",llm.StageNameWrite,cfg.FlowCompletionTokens,llm.ExecutionTextOnly}}{
  p,e:=r.FreezeExecution(t.Context(),llm.ModelRef{ProviderID:"openrouter",ModelID:v.model},v.stage,v.tokens,llm.ReasoningLow,v.delivery);if e!=nil{t.Logf("preflight %s/%s refused: %v",v.model,v.stage,e);continue};p.ResponseRetries=3;upper,ok:=p.QuoteMicrousd();if !ok{t.Fatal("missing frozen ceiling")};policies=append(policies,p);t.Logf("preflight %s/%s upperMicrousd=%d",v.model,v.stage,upper)
 };b,e:=json.MarshalIndent(policies,"","  ");if e!=nil{t.Fatal(e)};if e=os.WriteFile(filepath.Join(root,"live-policies.private.json"),b,0600);e!=nil{t.Fatal(e)};if len(policies)==0{t.Fatal(fmt.Errorf("no admitted live route"))}
}

type proofReceipt struct { Sequence int; Stage,Model,RequestDigest,BindingDigest,Status string; Upper,Measured int64; Usage llm.Usage }
type proofCostState struct { Version int; Cap,Confirmed,Unresolved int64; Halted bool; Calls []proofReceipt }
type privateProofModels struct { root string; registry *llm.Registry; lock *os.File; state proofCostState; mu sync.Mutex; binding string; evidence string }
func proofDigest(v any)string{b,e:=json.Marshal(v);if e!=nil{panic(e)};s:=sha256.Sum256(b);return hex.EncodeToString(s[:])}
func proofPersist(path string,v any)error{b,e:=json.MarshalIndent(v,"","  ");if e!=nil{return e};f,e:=os.OpenFile(path+".tmp",os.O_CREATE|os.O_EXCL|os.O_WRONLY,0600);if e!=nil{return e};if _,e=f.Write(b);e!=nil{f.Close();return e};if e=f.Sync();e!=nil{f.Close();return e};if e=f.Close();e!=nil{return e};if e=os.Rename(path+".tmp",path);e!=nil{return e};d,e:=os.Open(filepath.Dir(path));if e!=nil{return e};defer d.Close();return d.Sync()}
func newPrivateProofModels(t *testing.T,root string,r *llm.Registry)*privateProofModels{
 t.Helper();budget:=filepath.Join(root,"live-budget");if e:=os.MkdirAll(budget,0700);e!=nil{t.Fatal(e)};lock,e:=os.OpenFile(filepath.Join(budget,"parent.lock"),os.O_CREATE|os.O_EXCL|os.O_WRONLY,0600);if e!=nil{t.Fatal("another or uncertain live proof owns the budget")}
 m:=&privateProofModels{root:root,registry:r,lock:lock,state:proofCostState{Version:1,Cap:200000}};t.Cleanup(func(){lock.Close();os.Remove(filepath.Join(budget,"parent.lock"))})
 path:=filepath.Join(budget,"parent.json");if b,e:=os.ReadFile(path);e==nil{if json.Unmarshal(b,&m.state)!=nil||m.state.Cap!=200000||m.state.Halted||m.state.Unresolved!=0{t.Fatal("prior budget state refuses continuation")}}else if !os.IsNotExist(e){t.Fatal(e)}
 var a map[string]any;proofRead(t,filepath.Join(root,"spend-and-push-approval.json"),&a);m.evidence=proofDigest(a)
 if e:=proofPersist(path,m.state);e!=nil{t.Fatal(e)};return m
}
func(m *privateProofModels)Resolve(ref llm.ModelRef)(llm.ModelInfo,bool){return m.registry.Lookup(ref)}
func(m *privateProofModels)Complete(ctx context.Context,ref llm.ModelRef,req llm.Request)(llm.Response,error){
 m.mu.Lock();defer m.mu.Unlock();if req.Execution==nil||!req.Execution.Matches(ref,req)||m.state.Halted||m.state.Unresolved!=0||m.binding==""{return llm.Response{},analysisquality.ErrBudget}
 policy:=req.Execution.Call;upper,ok:=policy.QuoteMicrousd();remaining:=m.state.Cap-m.state.Confirmed;if !ok||upper>remaining||upper<0{return llm.Response{},analysisquality.ErrBudget}
 old,e:=m.lock.Stat();now,e2:=os.Lstat(filepath.Join(m.root,"live-budget","parent.lock"));if e!=nil||e2!=nil||!os.SameFile(old,now){return llm.Response{},analysisquality.ErrBudget}
 seq:=len(m.state.Calls);m.state.Unresolved=upper;parent:=filepath.Join(m.root,"live-budget","parent.json");if e=proofPersist(parent,m.state);e!=nil{m.state.Halted=true;return llm.Response{},e}
 path:=filepath.Join(m.root,"live-budget",fmt.Sprintf("call-%03d.json",seq));digest:=analysisquality.RequestDigest(req)
 approval:=analysisquality.Approval{SessionID:fmt.Sprintf("owner_clip_proof_%03d",seq),StatePath:path,ApprovedBy:"explicit owner approval in current conversation",EvidenceDigest:m.evidence,PlanDigest:m.binding,PolicyDigest:proofDigest(policy),ExpiresAt:time.Now().Add(2*time.Hour),MaximumMicrousd:&remaining,Calls:[]analysisquality.ApprovedCall{{ID:fmt.Sprintf("call-%03d",seq),RequestDigest:digest}}}
 model,allowed:=m.registry.Lookup(ref);if !allowed||model.Disabled||!model.ServesStage(req.Stage){m.state.Halted=true;_ = proofPersist(parent,m.state);return llm.Response{},analysisquality.ErrBudget}
 child,e:=analysisquality.OpenBudgetedModels(ctx,path,approval,policy,time.Now(),func(_ context.Context,a analysisquality.Approval)error{
  if os.Getenv("POSTPILOT_APPROVED_LIVE_CAP_MICROUSD")!="200000"||a.EvidenceDigest!=m.evidence||a.PlanDigest!=m.binding||a.Calls[0].RequestDigest!=digest||a.MaximumMicrousd==nil||*a.MaximumMicrousd!=remaining{return analysisquality.ErrBudget};return nil
 },func()(ai.Models,error){return proofRegistryModels{m.registry},nil});if e!=nil{m.state.Halted=true;_ = proofPersist(parent,m.state);return llm.Response{},e}
 response,callErr:=child.Complete(ctx,ref,req);closeErr:=child.Close()
 receipt:=proofReceipt{Sequence:seq,Stage:req.Stage,Model:ref.String(),RequestDigest:digest,BindingDigest:m.binding,Status:"completed",Upper:upper,Usage:response.Usage}
 if response.Usage.CostReported&&response.Usage.CostMicrousd>=0{receipt.Measured=response.Usage.CostMicrousd;m.state.Confirmed+=receipt.Measured;m.state.Unresolved=0}else{receipt.Status="usage_uncertain";m.state.Halted=true}
 if callErr!=nil||closeErr!=nil||receipt.Measured>upper||m.state.Confirmed>m.state.Cap{m.state.Halted=true;receipt.Status="halted"}
 m.state.Calls=append(m.state.Calls,receipt)
 raw:=map[string]any{"receipt":receipt,"response":response};if e=proofPersist(filepath.Join(m.root,"live-budget",fmt.Sprintf("response-%03d.json",seq)),raw);e!=nil{m.state.Halted=true;_ = proofPersist(parent,m.state);return response,e}
 if e=proofPersist(parent,m.state);e!=nil{m.state.Halted=true;return response,e}
 fmt.Printf("PROOF_CALL sequence=%d stage=%s actualMicrousd=%d cumulativeMicrousd=%d status=%s\n",seq,req.Stage,receipt.Measured,m.state.Confirmed,receipt.Status)
 if closeErr!=nil{return response,closeErr};if callErr!=nil{return response,callErr};if m.state.Halted{return response,analysisquality.ErrUncertain};return response,nil
}
type proofRegistryModels struct{*llm.Registry}
func(m proofRegistryModels)Resolve(ref llm.ModelRef)(llm.ModelInfo,bool){return m.Lookup(ref)}

type proofPayload struct{Language,Ratio,Disclosure,Instruction,CaptionPace,Accent,IntroPreset,OutroPreset string;HideDisclosure bool;TargetDurationMS int;Composition *clip.ProjectComposition;Template clip.Recipe;Guidelines clip.VideoGuidelines;FollowStoryline *clip.Storyline;Regions *clip.ProjectRegions;CaptionStyles []string}
type proofObservation struct{Arm string;Repetition,Index int;Analysis clip.ChunkAnalysis;Usage llm.Usage;Failure string}
func proofPlanning(p proofPayload,analyses []clip.SourceAnalysis,policy llm.CallPolicy,style string,retained clip.EditPlan)clip.PlanningInput{
 design:=clip.ProjectDesign{CaptionPace:p.CaptionPace,Accent:p.Accent,IntroPreset:p.IntroPreset,OutroPreset:p.OutroPreset,CaptionStyles:[]string{style}}
 in:=clip.PlanningInput{Language:p.Language,Ratio:p.Ratio,Disclosure:p.Disclosure,HideDisclosure:p.HideDisclosure,Instruction:p.Instruction,TargetDurationMS:p.TargetDurationMS,Composition:p.Composition,Template:p.Template,Guidelines:p.Guidelines,FollowStoryline:p.FollowStoryline,Regions:p.Regions,Design:design,Analyses:analyses,Policy:policy}
 if retained.SourceAudio!=nil{in.SourceAudio=retained.SourceAudio.Values};return in
}
func TestPrivateRealClipPipeline(t *testing.T){
 if os.Getenv("POSTPILOT_SEND_APPROVED_LIVE_CALLS")!="1"{t.Skip("explicit live execution flag absent")}
 root,registry:=proofRegistry(t);models:=newPrivateProofModels(t,root,registry);cfg:=ai.DefaultConfig(clip.Environment{});service,e:=ai.New(models,cfg);if e!=nil{t.Fatal(e)}
 observeRef:=llm.ModelRef{ProviderID:"openrouter",ModelID:"google/gemini-3.8-flash"};writeRef:=llm.ModelRef{ProviderID:"openrouter",ModelID:"qwen/qwen3.8-flash"}
 observe,e:=registry.FreezeExecution(t.Context(),observeRef,llm.StageNameObserve,cfg.ObserveCompletionTokens,cfg.ObserveReasoning,llm.ExecutionInlineStatic);if e!=nil{t.Fatal(e)};observe.ResponseRetries=3
 write,e:=registry.FreezeExecution(t.Context(),writeRef,llm.StageNameWrite,cfg.FlowCompletionTokens,cfg.PlanReasoning,llm.ExecutionTextOnly);if e!=nil{t.Fatal(e)};write.ResponseRetries=3
 var admission struct{Payload proofPayload `json:"approvedProductionPayload"`};proofRead(t,filepath.Join(root,"live-admission.private.json"),&admission)
 var domain struct{Analysis []clip.SourceAnalysis `json:"analysis"`;Plan json.RawMessage `json:"plan"`};proofRead(t,filepath.Join(root,"production-domain.json"),&domain);retained,e:=clip.DecodeEditPlan(string(domain.Plan));if e!=nil{t.Fatal(e)}
 var inputs []struct{ID,Fingerprint string};proofRead(t,filepath.Join(root,"playback-capabilities.private.json"),&inputs)
 var verified []struct{Arm string `json:"arm"`;Index int `json:"index"`;Duration int `json:"durationMs"`;SHA string `json:"sha256"`};proofRead(t,filepath.Join(root,"verified-analysis-copies.private.json"),&verified)
 var records []proofObservation;var allPlans []map[string]any
 observeArm:=func(arm string,repetition int,indices []int)([]clip.SourceAnalysis,error){var analyses []clip.SourceAnalysis
  for _,i:=range indices{var source clip.AnalysisSource;for _,a:=range domain.Analysis{if a.Source.ID==inputs[i].ID{source=a.Source;break}};if source.ID==""{return nil,errors.New("unknown authorized source")}
   var duration int;var expected string;for _,v:=range verified{if v.Arm==arm&&v.Index==i{duration=v.Duration;expected=v.SHA;break}};path:=filepath.Join(root,"private-media",fmt.Sprintf("%s-source-%d.mp4",arm,i));bytes,e:=os.ReadFile(path);if e!=nil{return nil,e};actual:=sha256.Sum256(bytes);if duration<=0||hex.EncodeToString(actual[:])!=expected||len(bytes)>8<<20{return nil,errors.New("artifact identity invalid")}
   models.binding=proofDigest(struct{Source clip.AnalysisSource;Artifact string;Duration int}{source,expected,duration})
   video:=llm.InlineVideo{MIME:"video/mp4",Size:int64(len(bytes)),DurationMS:int64(duration),Sampling:llm.VideoSamplingFixed,Open:func(context.Context)(io.ReadCloser,error){return os.Open(path)}}
   input:=clip.ChunkInput{Source:source,Language:admission.Payload.Language,DurationMS:duration,Policy:observe,Video:video}
   chunk,usage,e:=service.ObserveChunk(t.Context(),observeRef,input);record:=proofObservation{Arm:arm,Repetition:repetition,Index:i,Analysis:chunk,Usage:usage};if e!=nil{record.Failure=e.Error()};records=append(records,record)
   if e2:=proofPersist(filepath.Join(root,"live-observations.private.json"),records);e2!=nil{return nil,e2};if e!=nil{return nil,e};analyses=append(analyses,clip.SourceAnalysis{Source:source,Segments:chunk.Segments})
  };return analyses,nil
 }
 indices:=make([]int,len(inputs));for i:=range indices{indices[i]=i}
 for n,arm:=range []string{"browser","native"}{
  analyses,e:=observeArm(arm,1,indices);if e!=nil{t.Logf("pipeline%d analysis refused; see private receipts",n);break}
  style:=[]string{"bold","keynote"}[n];in:=proofPlanning(admission.Payload,analyses,write,style,retained);models.binding=proofDigest(in)
  flow,_,e:=service.Flow(t.Context(),writeRef,in);if e!=nil{t.Logf("pipeline%d flow refused; see private receipts",n);break};flow=flow.WithDesign(in.Design);flow.SourceAudio=retained.SourceAudio;if in.FollowStoryline!=nil{flow.Storyline=in.FollowStoryline};models.binding=proofDigest(struct{Input clip.PlanningInput;Flow clip.EditPlan}{in,flow})
  plan,_,e:=service.Narrate(t.Context(),writeRef,clip.NarrationInput{PlanningInput:in,Flow:flow});if e!=nil{t.Logf("pipeline%d caption planning refused; see private receipts",n);break};plan=plan.WithDesign(in.Design);plan.SourceAudio=retained.SourceAudio;clip.RestrictGeneratedCaptionStyles(&plan,in.Design.AllowedCaptionStyles())
  if admission.Payload.Regions!=nil{project:=clip.Project{Regions:admission.Payload.Regions};drafts:=[]clip.RegionDraft{};if plan.Storyline!=nil{drafts=plan.Storyline.RegionDrafts};plan,_,e=clip.ProjectPlanRegions(plan,clip.WrittenRegions(project,drafts),in.Design.RegionPresets());if e!=nil{t.Logf("pipeline%d region projection refused",n);break}}
  encoded,e:=clip.EncodeEditPlan(plan);if e!=nil{t.Fatal(e)};name:=fmt.Sprintf("proof-plan-%d",n);if e=proofPersist(filepath.Join(root,name+".private.json"),map[string]any{"plan":json.RawMessage(encoded),"analysis":analyses,"design":in.Design,"ratio":in.Ratio});e!=nil{t.Fatal(e)}
  allPlans=append(allPlans,map[string]any{"index":n,"arm":arm,"style":style,"durationMs":plan.DurationMS,"cuts":len(plan.Cuts),"textElements":len(plan.Portable.Elements)});if e=proofPersist(filepath.Join(root,"live-plan-index.private.json"),allPlans);e!=nil{t.Fatal(e)};t.Logf("pipeline%d plan complete cuts=%d durationMs=%d",n,len(plan.Cuts),plan.DurationMS)
 }
 if !models.state.Halted{for _,arm:=range []string{"reference","native","browser"}{rep:=2;if arm=="reference"{rep=1};if _,e:=observeArm(arm,rep,[]int{0,3,6});e!=nil{t.Log("comparison stopped at an owned parser/budget gate");break}}}
 if len(allPlans)<2{t.Fatal("two real pipeline plans not yet completed; private receipts retained")}
 t.Logf("real plans=%d providerCalls=%d confirmedMicrousd=%d unresolvedMicrousd=%d",len(allPlans),len(models.state.Calls),models.state.Confirmed,models.state.Unresolved)
}
