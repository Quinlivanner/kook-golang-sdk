package main

import (
	"bytes"
	. "github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v2"
	"os"
	"path/filepath"
	"regexp"
	"unicode"
)

type EventConfig struct {
	TypeAlias map[string]string `yaml:"type_alias"`
	Templates map[string]struct {
		Body map[string]string `yaml:"body"`
	} `yaml:"templates"`
	Events map[string]struct {
		Name     string             `yaml:"name"`
		Template *string            `yaml:"template"`
		Body     *map[string]string `yaml:"body"`
	} `yaml:"events"`
}

var events = &EventConfig{}

func main() {
	buf := &bytes.Buffer{}
	y, err := os.Open("events.yaml")
	if err != nil {
		panic(err)
	}
	defer y.Close()
	buf.ReadFrom(y)
	err = yaml.Unmarshal(buf.Bytes(), events)
	if err != nil {
		panic(err)
	}
	buf.Reset()
	f := NewFile("kook")
	f.Comment(`Code generated by tools/cmd/eventhandler.
Don't edit it.`)
	f.Comment("revive:disable")

	for typeName, item := range events.Templates {
		var c []Code
		for key, value := range item.Body {
			if v, ok := events.TypeAlias[value]; ok {
				value = v
			}
			if _, ok := events.Templates[value]; ok {
				value = "Event" + value
			}
			c = append(c, Id(key).Id(value).Tag(map[string]string{
				"json": convertKeyToJsonKey(key),
			}))
		}
		f.Type().Id("Event" + typeName).Struct(c...)
	}
	var initCode []Code
	var caseCode []Code
	for sysEvent, item := range events.Events {
		f.Type().Id(item.Name + "EventHandler").Func().Params(Id("*" + item.Name + "Context"))
		f.Func().Params(Id("eh").Id(item.Name + "EventHandler")).
			Id("Type").Params().String().Block(Return(Lit(sysEvent)))
		f.Func().Params(Id("eh").Id(item.Name + "EventHandler")).
			Id("New").Params().Id("EventContext").
			Block(Return(Id("&" + item.Name + "Context{}")))
		f.Func().Params(Id("eh").Id(item.Name + "EventHandler")).
			Id("Handle").Params(Id("i").Id("EventContext")).
			Block(If(List(Id("t"), Id("ok")).Op(":=").Id("i").Assert(Id("*"+item.Name+"Context")), Id("ok")).
				Block(Id("eh").Call(Id("t"))))
		if item.Template != nil {
			if _, ok := events.Templates[*item.Template]; ok {
				*item.Template = "Event" + *item.Template
			}
			f.Type().Id(item.Name+"Context").Struct(Id("*EventHandlerCommonContext"),
				Id("Extra").Id(*item.Template),
			)
		} else if item.Body != nil {
			var c []Code
			for key, value := range *item.Body {
				if v, ok := events.TypeAlias[value]; ok {
					value = v
				}
				if _, ok := events.Templates[value]; ok {
					value = "Event" + value
				}
				c = append(c, Id(key).Id(value).Tag(map[string]string{
					"json": convertKeyToJsonKey(key),
				}))
			}
			f.Type().Id(item.Name+"Context").Struct(Id("*EventHandlerCommonContext"),
				Id("Extra").Struct(c...))
		}
		f.Func().Params(Id("ctx").Id("*" + item.Name + "Context")).
			Id("GetExtra").Params().Interface().Block(
			Return(Id("&ctx.Extra")),
		)
		f.Func().Params(Id("ctx").Id("*"+item.Name+"Context")).
			Id("GetCommon").Params().Id("*EventHandlerCommonContext").
			Block(If(Id("ctx").Dot("EventHandlerCommonContext").Op("==").Nil()).
				Block(Id("ctx").Dot("EventHandlerCommonContext").Op("=").New(Id("EventHandlerCommonContext"))),
				Return(Id("ctx").Dot("EventHandlerCommonContext")))
		initCode = append(initCode, Id("registerEventHandler").Call(Id(item.Name+"EventHandler").Call(Nil())))
		caseCode = append(caseCode, Case(Func().Params(Id("*"+item.Name+"Context"))).Block(Return(Id(item.Name+"EventHandler").Call(Id("v")))))
	}

	f.Func().Id("init").Params().Block(
		initCode...,
	)
	f.Func().Id("handlerForInterface").Params(Id("i").Interface()).Id("EventHandler").Block(
		Switch(Id("v").Op(":=").Id("i").Assert(Id("type")).Block(caseCode...)),
		Return(Nil()),
	)
	f.Render(buf)
	r := regexp.MustCompile("// revive")
	replaced := r.ReplaceAll(buf.Bytes(), []byte("\n//revive"))
	o, err := os.OpenFile(filepath.Join("..", "..", "..", "eventhandlers.gen.go"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0777)
	if err != nil {
		panic(err)
	}
	defer o.Close()
	o.Write(replaced)

}

var matchId = regexp.MustCompile(`^([a-zA-z]*)(ID)$`)

func convertKeyToJsonKey(k string) string {
	s := matchId.ReplaceAllString(k, `${1}Id`)

	return toSnakeCase(s)
}

// ref: https://gist.github.com/stoewer/fbe273b711e6a06315d19552dd4d33e6
func toSnakeCase(s string) string {
	var res = make([]rune, 0, len(s))
	var p = '_'
	for i, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			res = append(res, '_')
		} else if unicode.IsUpper(r) && i > 0 {
			if unicode.IsLetter(p) && !unicode.IsUpper(p) || unicode.IsDigit(p) {
				res = append(res, '_', unicode.ToLower(r))
			} else {
				res = append(res, unicode.ToLower(r))
			}
		} else {
			res = append(res, unicode.ToLower(r))
		}

		p = r
	}
	return string(res)
}