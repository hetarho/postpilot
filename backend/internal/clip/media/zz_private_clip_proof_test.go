package media
import("testing";"os";"path/filepath";"encoding/json";"fmt";"time";"crypto/sha256";"encoding/hex";"github.com/postpilot/backend/internal/clip")
func TestPrivateNativeAndVerificationProof(t *testing.T){
 root:=os.Getenv("POSTPILOT_PRIVATE_PROOF_ROOT");if root==""{t.Skip("explicit private proof only")};root,_=filepath.EvalSymlinks(root)
 var domain struct{Analysis []clip.SourceAnalysis `json:"analysis"`};data,e:=os.ReadFile(filepath.Join(root,"production-domain.json"));if e!=nil{t.Fatal(e)};if e=json.Unmarshal(data,&domain);e!=nil{t.Fatal(e)}
 var access []struct{ID string `json:"id"`;Fingerprint string `json:"fingerprint"`};data,_=os.ReadFile(filepath.Join(root,"playback-capabilities.private.json"));if e=json.Unmarshal(data,&access);e!=nil{t.Fatal(e)}
 var browser struct{Result []struct{I int `json:"i"`;Slot struct{DurationMs,Width,Height int;HasAudio bool} `json:"slot"`}};data,_=os.ReadFile(filepath.Join(root,"browser-analysis-copies.private.json"));if e=json.Unmarshal(data,&browser);e!=nil{t.Fatal(e)}
 var report []map[string]any
 for _,arm:=range []string{"native","reference"}{
  cfg:=clip.DefaultMediaConfig(clip.Environment{WorkRoot:filepath.Join(root,"media-work-"+arm),FFmpegPath:"/opt/homebrew/bin/ffmpeg",FFprobePath:"/opt/homebrew/bin/ffprobe",WorkStaleAge:time.Hour,MediaTimeout:2*time.Minute})
  if arm=="reference"{cfg.CRF=12;cfg.VideoMaxRate=3000000;cfg.VideoBufferSize=6000000}
  a,e:=New(cfg,nil);if e!=nil{t.Fatal(e)}
  for i,v:=range access{if e=a.WithWorkspace(t.Context(),fmt.Sprintf("proof-%s-%d",arm,i),func(ws clip.MediaWorkspace)error{
   original:=filepath.Join(ws.Path,"original.mp4");b,err:=os.ReadFile(filepath.Join(root,"private-media",fmt.Sprintf("source-%d.mp4",i)));if err!=nil{return err};if err=os.WriteFile(original,b,0600);err!=nil{return err}
   info,err:=a.ProbeContainer(t.Context(),ws,original);if err!=nil{return err}
   source:=clip.MediaSource{Path:original,SourceID:v.ID,Fingerprint:v.Fingerprint,Info:info};began:=time.Now()
   measured,err:=a.PrepareAnalysisChunks(t.Context(),ws,source,func(chunk clip.AnalysisChunk)error{
    b,err:=os.ReadFile(chunk.Path);if err!=nil{return err};digest:=sha256.Sum256(b);p:=filepath.Join(root,"private-media",fmt.Sprintf("%s-source-%d.mp4",arm,i));if err=os.WriteFile(p,b,0600);err!=nil{return err}
    check:=clip.AnalysisCopy{Slot:fmt.Sprintf("%s-%d",arm,i),DurationMS:chunk.DurationMS,Width:browser.Result[i].Slot.Width,Height:browser.Result[i].Slot.Height,HasAudio:info.HasAudio,Bytes:int64(len(b)),Digest:hex.EncodeToString(digest[:])}
    verification,err:=a.VerifyAnalysisCopy(t.Context(),ws,chunk.Path,check);if err!=nil{return fmt.Errorf("%s copy%d verification: %w",arm,i,err)}
    report=append(report,map[string]any{"arm":arm,"index":i,"bytes":len(b),"sha256":check.Digest,"durationMs":chunk.DurationMS,"verification":verification});return nil
   });if err!=nil{return err};_ = measured
   report[len(report)-1]["elapsedMs"]=time.Since(began).Milliseconds()
   if arm=="native"{
    b,err=os.ReadFile(filepath.Join(root,"private-media",fmt.Sprintf("browser-source-%d.mp4",i)));if err!=nil{return err};p:=filepath.Join(ws.Path,"browser.mp4");if err=os.WriteFile(p,b,0600);err!=nil{return err};digest:=sha256.Sum256(b);slot:=browser.Result[i].Slot
    check:=clip.AnalysisCopy{Slot:fmt.Sprintf("browser-%d",i),DurationMS:slot.DurationMs,Width:slot.Width,Height:slot.Height,HasAudio:slot.HasAudio,Bytes:int64(len(b)),Digest:hex.EncodeToString(digest[:])}
    verification,err:=a.VerifyAnalysisCopy(t.Context(),ws,p,check);if err!=nil{return fmt.Errorf("browser copy%d verification: %w",i,err)}
    report=append(report,map[string]any{"arm":"browser","index":i,"bytes":len(b),"sha256":check.Digest,"durationMs":slot.DurationMs,"verification":verification})
   };return nil
  });e!=nil{t.Fatal(e)}}
 }
 data,e=json.MarshalIndent(report,"","  ");if e!=nil{t.Fatal(e)};if e=os.WriteFile(filepath.Join(root,"verified-analysis-copies.private.json"),data,0600);e!=nil{t.Fatal(e)};t.Logf("verified %d actual analysis artifacts",len(report))
}
