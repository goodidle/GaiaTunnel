//shell dispatch tunnel
//
package tunnel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"syscall"
	"time"
)

/*
 * State: 0-prepare,1-SavErr,2-Saved,3-RoleErr,4-StartErr,5-running,6-timeout,7-failed,8-killed,9-success
 * Args: Shell's args
 * PoolAttr: pool's attr
 * GlobalArgs: global args, for multi shells task, such as: tag...
 * Agent: agent's art, such as: version, local ip...
 */
type Shell struct {
	Id					int64					`json:"id"`
	Idx					string					`json:"idx"` //字符串版Id，防止JS解析时末尾3个数字为0
	Content				string					`json:"content"`
	Timeout				time.Duration			`json:"timeout"`
	Role				string					`json:"role"`
	Args				[]Arg					`json:"args"`
	PoolAttr			[]Arg					`json:"pool_attr"`
	GlobalArgs			[]Arg					`json:"global_args"`
	Agent				[]Arg					`json:"agent"`
	State				int64					`json:"state"`
	Pid					int						`json:"pid"`
	Path				string					`json:"path"`
	BeginTime			time.Time				`json:"begin_time"`
	EndTime				time.Time				`json:"end_time"`
	User				*user.User				`json:"-"`
}

type Arg struct {
	Key			string		`json:"key"`
	Value	 	string		`json:"value"`
}

var (
	MinTimeout 	time.Duration	= 5
	DefTimeout	time.Duration	= 10
	MaxTimeout	time.Duration	= 0
	DefPath 					= "./logs"

	NotRunning					= errors.New("NotRunning")
	TimeExpired					= errors.New("TimeExpired")
	SignKilled					= errors.New("signal: killed")
)

func (o Shell) String() string {
	s, _ := json.Marshal(o)
	return string(s)
}

func (o *Shell) Init()  {
	o.GetUUID()
	o.Pid = 0
	o.State = 0
	o.BeginTime = time.Now()
	o.EndTime = time.Now()
	if o.Timeout < MinTimeout {
		o.Timeout = DefTimeout
	}
	if MaxTimeout > 0 {
		if o.Timeout > MaxTimeout {
			o.Timeout = MaxTimeout
		}
	}
	if o.Path == "" {
		o.Path = DefPath
	}
	if o.Role == "" {
		o.Role = "root"
	}
}

func (o *Shell) SetPath(Path string)  {
	o.Path = strings.TrimSpace(Path)
	if o.Path == "" {
		o.Path = DefPath
	}
	FileInfo, err := os.Stat(o.Path)
	if err != nil || ! FileInfo.IsDir() {
		err = os.MkdirAll(o.Path, 0755)
		if err != nil {
			log.Printf("Create Path %s, err: %s", o.Path, err.Error())
		}else{
			log.Printf("Create Path %s, success", o.Path)
		}
	}
}

func (o *Shell) GetUUID() {
	t := time.Now()
	o.Id = t.UnixNano()
	o.Idx = fmt.Sprintf("%d", o.Id)
}

func (o *Shell) CheckRole() bool {
	Curr, err := user.Current()
	if err != nil {
		fmt.Printf("Shell: %d, Check Role, get curr user, err: %s\n", o.Id, err.Error())
		return false
	}
	if Curr.Uid != "0" {
		fmt.Printf("Shell: %d, Check Role, curr user must be root, err: not root\n", o.Id)
		return false
	}
	User, err := user.Lookup(o.Role)
	if err != nil {
		fmt.Printf("Shell: %d, Check Role err: %s\n", o.Id, err.Error())
		o.Finish(3)
		return false
	}
	o.User = User
	return true
}

func (o *Shell) FileName() string {
	return fmt.Sprintf("%s/%d", o.Path, o.Id)
}

func (o *Shell) OutputFileName() string {
	return fmt.Sprintf("%s/%d.log", o.Path, o.Id)
}

func (o *Shell) Save() bool {
	File, err := os.OpenFile(o.FileName(), os.O_CREATE|os.O_RDWR|os.O_APPEND, 0755)
	defer File.Close()
	if err != nil {
		o.Finish(1)
		fmt.Printf("Shell: %d, Create Shell err: %s\n", o.Id, err.Error())
		return false
	}
	_, err = File.WriteString(o.Content)
	if err != nil {
		o.Finish(1)
		fmt.Printf("Shell: %d, Save Shell err: %s\n", o.Id, err.Error())
		return false
	}
	o.Finish(2)
	return true
}

func (o *Shell) Start() bool {
	o.BeginTime = time.Now()
	if ! o.Save() {
		return false
	}
	Output, _ := os.Create(o.OutputFileName())
	defer Output.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second * o.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", o.FileName())
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if o.Role != "root" {
		if ! o.CheckRole() {
			return false
		}
		uid, _ := strconv.Atoi(o.User.Uid)
		gid, _ := strconv.Atoi(o.User.Gid)
		cmd.SysProcAttr.Credential = &syscall.Credential{
			Uid: uint32(uid),
			Gid: uint32(gid),
		}
	}
	if len(o.PoolAttr) > 0 {
		for _, arg := range o.PoolAttr {
			cmd.Env = append(cmd.Env,
				fmt.Sprintf("APP_%s=%s", strings.ToUpper(arg.Key), strings.TrimSpace(arg.Value)),
				fmt.Sprintf("POOL_%s=%s", strings.ToUpper(arg.Key), strings.TrimSpace(arg.Value)))
		}
	}
	if len(o.GlobalArgs) > 0 {
		for _, arg := range o.GlobalArgs {
			cmd.Env = append(cmd.Env,
				fmt.Sprintf("APP_%s=%s", strings.ToUpper(arg.Key), strings.TrimSpace(arg.Value)),
				fmt.Sprintf("GLOBAL_%s=%s", strings.ToUpper(arg.Key), strings.TrimSpace(arg.Value)))
		}
	}
	if len(o.Args) > 0 {
		for _, arg := range o.Args {
			cmd.Env = append(cmd.Env,
				fmt.Sprintf("APP_TASK_%s=%s", strings.ToUpper(arg.Key), strings.TrimSpace(arg.Value)),
				fmt.Sprintf("SH_ARG_%s=%s", strings.ToUpper(arg.Key), strings.TrimSpace(arg.Value)))
		}
	}
	if len(o.Agent) > 0 {
		for _, arg := range o.Agent {
			cmd.Env = append(cmd.Env,
				fmt.Sprintf("AGENT_%s=%s", strings.ToUpper(arg.Key), strings.TrimSpace(arg.Value)))
		}
	}
	cmd.Stdout = Output
	cmd.Stderr = Output

	err := cmd.Start()
	if err != nil {
		fmt.Printf("Shell: %d, Start Shell err: %s\n", o.Id, err.Error())
		o.Finish(4)
		return false
	}
	o.Finish(5)
	o.Pid = cmd.Process.Pid

	o.Waiting(ctx, cmd)
	return true
}

//Stop Running - not timeout means user demand kill it
func (o *Shell) Stop() error {
	if o.IsFinish() {
		return NotRunning
	}
	if time.Now().Unix() - o.BeginTime.Unix() > 3600 {
		return TimeExpired
	}
	if o.Pid == 0 {
		return NotRunning
	}
	err := syscall.Kill(-o.Pid, syscall.SIGKILL)
	if err != nil {
		fmt.Printf("Shell: %d, user demand kill it, err: %s.\n", o.Id, err.Error())
		return err
	}
	fmt.Printf("Shell: %d, user demand kill it.\n", o.Id)
	return nil
}

func (o *Shell) Finish(State int64)  {
	o.State = State
	o.EndTime = time.Now()
	o.Content = ""
	if o.IsFinish() || o.State == 5 {
		o.Args = nil
		o.PoolAttr = nil
		o.GlobalArgs = nil
		o.Agent = nil
	}
}

func (o *Shell) IsRunning() bool {
	return o.State == 0 || o.State == 2 || o.State == 5
}

func (o *Shell) IsFinish() bool {
	return o.State == 1 || o.State == 3 || o.State == 4 || o.State > 5
}

func (o *Shell) Output() ([]byte, error) {
	return ioutil.ReadFile(o.OutputFileName())
}

func (o *Shell) OutputString() (string, error) {
	Bytes, err := o.Output()
	return string(Bytes), err
}

//Remove Shell and Log file
func (o *Shell) Remove() {
	err := os.Remove(o.FileName())
	if err != nil {
		fmt.Printf("Shell: %d, Remove Shell err: %s\n", o.Id, err.Error())
	}
	err = os.Remove(o.OutputFileName())
	if err != nil {
		fmt.Printf("Shell: %d, Remove Output err: %s\n", o.Id, err.Error())
	}
}

//Waiting Shell Finish
func (o *Shell) Waiting(ctx context.Context, cmd *exec.Cmd) {
	WaitChan := make(chan struct{}, 1)
	defer close(WaitChan)

	go func() {
		select {
		case <-ctx.Done():
			o.Finish(6)
			fmt.Printf("Shell: %d, Shell run timeout.\n", o.Id)
		case <-WaitChan:
		}
	}()

	err := cmd.Wait()
	WaitChan <- struct{}{}
	if err != nil {
		if err.Error() == SignKilled.Error() {
			if o.State != 6 {
				fmt.Printf("Shell: %d, Shell was killed by user.\n", o.Id)
				o.Finish(8)
			}
		}else{
			o.Finish(7)
		}
	}else{
		o.Finish(9)
	}
}

func NewShell(Id int64) *Shell {
	if Id <= 0 {
		return nil
	}
	return &Shell{
		Id:	Id,
	}
}