package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	qrterminal "github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
	_ "modernc.org/sqlite"
	_ "time/tzdata"
)

const janelaEdicao = 14 * time.Minute

type Bot struct {
	cli    *whatsmeow.Client
	mu     sync.Mutex
	grupos map[string]*Grupo
	dir    string
	fuso   *time.Location
	inicio time.Time
}

func main() {
	dir := os.Getenv("VOLEI_DATA")
	if dir == "" {
		dir = "dados"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fatal(err)
	}

	nomeFuso := os.Getenv("VOLEI_TZ")
	if nomeFuso == "" {
		nomeFuso = "America/Sao_Paulo"
	}
	fuso, err := time.LoadLocation(nomeFuso)
	if err != nil {
		fatal(err)
	}

	ctx := context.Background()
	dsn := "file:" + filepath.Join(dir, "sessao.db") + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		fatal(err)
	}
	db.SetMaxOpenConns(1)

	nivel := os.Getenv("VOLEI_LOG")
	if nivel == "" {
		nivel = "WARN"
	}
	container := sqlstore.NewWithDB(db, "sqlite", waLog.Stdout("DB", nivel, true))
	if err := container.Upgrade(ctx); err != nil {
		fatal(err)
	}
	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		fatal(err)
	}

	bot := &Bot{
		grupos: map[string]*Grupo{},
		dir:    dir,
		fuso:   fuso,
		inicio: time.Now(),
	}
	if err := bot.Carregar(); err != nil {
		fatal(err)
	}

	bot.cli = whatsmeow.NewClient(device, waLog.Stdout("WA", nivel, true))
	bot.cli.AddEventHandler(bot.handler)

	if bot.cli.Store.ID == nil {
		qrChan, err := bot.cli.GetQRChannel(ctx)
		if err != nil {
			fatal(err)
		}
		if err := bot.cli.Connect(); err != nil {
			fatal(err)
		}
		for evt := range qrChan {
			switch evt.Event {
			case "code":
				fmt.Println("\n📱 WhatsApp > Aparelhos conectados > Conectar aparelho\nEscaneia esse QR:")
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
			case "success":
				fmt.Println("✅ Conectado!")
			default:
				fmt.Println("QR:", evt.Event)
			}
		}
	} else if err := bot.cli.Connect(); err != nil {
		fatal(err)
	}

	fmt.Println("🏐 bot do vôlei de hoje no ar. manda !volei no grupo. ctrl+c pra sair.")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	bot.cli.Disconnect()
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "erro:", err)
	os.Exit(1)
}

func (b *Bot) handler(raw any) {
	switch evt := raw.(type) {
	case *events.Message:
		b.aoReceber(evt)
	case *events.Connected:
		fmt.Println("conectado ao whatsapp")
	case *events.LoggedOut:
		fmt.Println("sessão encerrada no celular. apague dados/sessao.db e escaneie o QR de novo.")
		os.Exit(1)
	}
}

func desembrulha(msg *waE2E.Message) *waE2E.Message {
	for i := 0; i < 4 && msg != nil; i++ {
		switch {
		case msg.GetEphemeralMessage().GetMessage() != nil:
			msg = msg.GetEphemeralMessage().GetMessage()
		case msg.GetViewOnceMessage().GetMessage() != nil:
			msg = msg.GetViewOnceMessage().GetMessage()
		case msg.GetViewOnceMessageV2().GetMessage() != nil:
			msg = msg.GetViewOnceMessageV2().GetMessage()
		default:
			return msg
		}
	}
	return msg
}

func textoDe(msg *waE2E.Message) string {
	if msg == nil {
		return ""
	}
	if c := msg.GetConversation(); c != "" {
		return c
	}
	if e := msg.GetExtendedTextMessage(); e != nil {
		return e.GetText()
	}
	return ""
}

func (b *Bot) hoje() time.Time {
	agora := time.Now().In(b.fuso)
	return time.Date(agora.Year(), agora.Month(), agora.Day(), 0, 0, 0, 0, b.fuso)
}

func (b *Bot) grupo(chat string) *Grupo {
	g, ok := b.grupos[chat]
	if !ok {
		g = &Grupo{}
		b.grupos[chat] = g
	}
	return g
}

func (b *Bot) aoReceber(evt *events.Message) {
	if evt.Info.Timestamp.Before(b.inicio) {
		return
	}
	msg := desembrulha(evt.Message)
	texto := strings.TrimSpace(textoDe(msg))
	campos := strings.Fields(texto)
	if len(campos) == 0 {
		return
	}
	cmd := strings.ToLower(campos[0])
	switch cmd {
	case "!eu", "!nao", "!não", "!volei", "!vôlei", "!abortarmissao", "!abortarmissão", "!add", "!remove", "!talvez":
	default:
		return
	}
	resto := strings.TrimSpace(texto[len(campos[0]):])

	chat := evt.Info.Chat
	if chat.Server != types.GroupServer {
		b.responder(chat, "Isso só funciona dentro do grupo. 🏐")
		return
	}

	ctx := context.Background()
	info, err := b.cli.GetGroupInfo(ctx, chat)
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro ao buscar o grupo:", err)
		b.responder(chat, "⚠️ Não consegui ver quem está no grupo agora, tenta de novo daqui a pouco.")
		return
	}
	todos, eu := b.membros(ctx, info.Participants)
	nome := evt.Info.PushName
	if nome == "" {
		nome = b.nomeDe(ctx, evt.Info.Sender, evt.Info.SenderAlt)
	}

	b.mu.Lock()
	g := b.grupo(chat.String())
	hoje := b.hoje()
	v := g.Atual(hoje)
	membros := g.Ajustar(todos)
	var aviso string
	abortou := cmd == "!abortarmissao" || cmd == "!abortarmissão"
	switch {
	case abortou:
		v = g.Agendar(v, "", hoje)
		v.Zerar()
	case cmd == "!add" || cmd == "!remove":
		nomes := nomesDe(resto)
		if len(nomes) == 0 {
			aviso = fmt.Sprintf("Manda o nome: `%s Fulano`, ou `%s \"Jose Maria\"` pra nome composto.", cmd, cmd)
		}
		var erros []string
		for _, n := range nomes {
			var erro string
			if cmd == "!add" {
				erro = g.Incluir(membros, n)
			} else {
				erro = g.Tirar(v, membros, n)
			}
			if erro != "" {
				erros = append(erros, erro)
			}
			membros = g.Ajustar(todos)
		}
		if len(erros) > 0 {
			aviso = strings.Join(erros, "\n")
		}
		if len(erros) == len(nomes) {
			v = nil
		}
	case cmd == "!volei" || cmd == "!vôlei":
		campos := strings.Fields(resto)
		if len(campos) == 0 {
			break
		}
		if dia, erro, ok := LerData(campos[len(campos)-1], hoje); ok {
			if erro != "" {
				aviso, v = erro, nil
				break
			}
			nome := strings.TrimFunc(normaliza(strings.Join(campos[:len(campos)-1], " ")), ehAspas)
			v = g.Agendar(v, normaliza(nome), dia)
			break
		}
		switch strings.ToLower(campos[0]) {
		case "zerar":
			v.Zerar()
		case "config":
			aviso, v = g.Config(), nil
		}
	default:
		r := Vai
		switch cmd {
		case "!nao", "!não":
			r = NaoVai
		case "!talvez":
			r = Talvez
		}
		if resto == "" {
			v.Marcar(idsDoRemetente(evt.Info, info.Participants), nome, r)
		} else {
			ids, erro := v.Encontrar(membros, resto)
			if erro != "" {
				aviso, v = erro, nil
			} else {
				v.Marcar(ids, "", r)
			}
		}
	}
	saida, chamada := "", ""
	switch {
	case v == nil:
	case abortou:
		saida, chamada = mensagemAbortada, "*Missão abortada*"
	default:
		saida, chamada = v.Render(membros, hoje), v.Chamada()
	}
	b.mu.Unlock()

	if aviso != "" {
		b.responder(chat, aviso)
	}
	if v != nil {
		b.publicar(ctx, chat, eu, v, saida, chamada)
	}
}

func (b *Bot) publicar(ctx context.Context, chat, eu types.JID, v *Votacao, texto, chamada string) {
	resp, err := b.cli.SendMessage(ctx, chat, &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String(texto)},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro ao enviar:", err)
	}

	b.mu.Lock()
	var antigas []string
	var numero int
	if err == nil {
		antigas, numero = v.Registrar(resp.ID, time.Now(), time.Now().Add(-janelaEdicao))
	}
	if err := b.Salvar(); err != nil {
		fmt.Fprintln(os.Stderr, "erro ao salvar:", err)
	}
	b.mu.Unlock()

	if len(antigas) == 0 {
		return
	}
	hora := time.Now().In(b.fuso).Format("15:04")
	aponta := &waE2E.ExtendedTextMessage{
		Text: proto.String(fmt.Sprintf("⬇️ %s mais abaixo — #%d às %s.", chamada, numero, hora)),
	}
	if !eu.IsEmpty() {
		aponta.ContextInfo = &waE2E.ContextInfo{
			StanzaID:      proto.String(resp.ID),
			Participant:   proto.String(eu.String()),
			QuotedMessage: &waE2E.Message{Conversation: proto.String(texto)},
		}
	}
	for _, id := range antigas {
		edit := b.cli.BuildEdit(chat, id, &waE2E.Message{ExtendedTextMessage: aponta})
		if _, err := b.cli.SendMessage(ctx, chat, edit); err != nil {
			fmt.Fprintln(os.Stderr, "erro ao editar:", err)
		}
	}
}

func (b *Bot) membros(ctx context.Context, participantes []types.GroupParticipant) ([]Membro, types.JID) {
	var proprios []string
	if id := b.cli.Store.ID; id != nil {
		proprios = append(proprios, id.User)
	}
	if lid := b.cli.Store.LID; !lid.IsEmpty() {
		proprios = append(proprios, lid.User)
	}

	var eu types.JID
	var out []Membro
	for _, p := range participantes {
		ids := usuarios(p.JID, p.LID, p.PhoneNumber)
		m := Membro{IDs: ids, Nome: b.nomeDe(ctx, p.JID, p.PhoneNumber, p.LID)}
		if temAlgum(ids, proprios) {
			m.Proprio = true
			eu = p.JID.ToNonAD()
		}
		out = append(out, m)
	}
	return out, eu
}

func (b *Bot) nomeDe(ctx context.Context, jids ...types.JID) string {
	var redigido string
	for _, j := range jids {
		if j.IsEmpty() {
			continue
		}
		c, err := b.cli.Store.Contacts.GetContact(ctx, j.ToNonAD())
		if err != nil || !c.Found {
			continue
		}
		for _, n := range []string{c.PushName, c.FullName, c.FirstName, c.BusinessName} {
			if n = strings.TrimSpace(n); n != "" {
				return n
			}
		}
		if redigido == "" {
			redigido = c.RedactedPhone
		}
	}
	for _, j := range jids {
		if j.Server == types.DefaultUserServer {
			return "+" + j.User
		}
	}
	if redigido != "" {
		return redigido
	}
	return "Sem nome"
}

func idsDoRemetente(info types.MessageInfo, participantes []types.GroupParticipant) []string {
	ids := usuarios(info.Sender, info.SenderAlt)
	for _, p := range participantes {
		if temAlgum(usuarios(p.JID, p.LID, p.PhoneNumber), ids) {
			return usuarios(info.Sender, info.SenderAlt, p.JID, p.LID, p.PhoneNumber)
		}
	}
	return ids
}

func usuarios(jids ...types.JID) []string {
	var out []string
	for _, j := range jids {
		if j.User != "" && !contem(out, j.User) {
			out = append(out, j.User)
		}
	}
	return out
}

func (b *Bot) responder(chat types.JID, texto string) {
	_, err := b.cli.SendMessage(context.Background(), chat, &waE2E.Message{
		Conversation: proto.String(texto),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro ao enviar:", err)
	}
}
