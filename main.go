package main

import (
	"flag"
	"fmt"
	"github.com/Jeffail/gabs/v2"
	"io/fs"
	"log"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
)

// We load a map of all user id's and display names into memory.  This is fine for the workspaces I've
// administered, but I suppose it could fail on workspaces with an enormous number of users.
// Good Enough (tm) for now...
var users = make(map[string]string)
var channels = make(map[string]string)
var inDir string
var outDir string
var title string

func main() {
	flag.StringVar(&inDir, "in", ".", "Unzipped location of the Slack export")
	flag.StringVar(&outDir, "out", ".", "Directory where to write output files")
	flag.StringVar(&title, "title", "", "Title")

	flag.Parse()

	var err error

	if err = getUsers(); err != nil {
		log.Fatal(err)
	}
	if err = getChannels(); err != nil {
		log.Fatal(err)
	}

	if err = processIndex(); err != nil {
		log.Fatal(err)
	}

	if err = processChannels(); err != nil {
		log.Fatal(err)
	}
}

func parseList(body []byte, descr string, listName string, keyName string, valueName string) (result map[string]string) {
	jsonParsed, err := gabs.ParseJSON(body)
	if err != nil {
		log.Fatal("FATAL: parse ", descr, ": ", string(body), ": ", err)
	}

	list := jsonParsed.Search(listName).Children()

	result = make(map[string]string)
	for _, m := range list {
		k := m.Search(keyName).Data().(string)
		v := m.Search(valueName).Data().(string)
		result[k] = v
	}

	return
}

func parseArray(body []byte, descr string, keyName string, valueName string, fallbackValueName string) (result map[string]string) {
	jsonParsed, err := gabs.ParseJSON(body)
	if err != nil {
		log.Fatal("FATAL: parse ", descr, ": ", string(body), ": ", err)
	}

	list := jsonParsed.Children()

	result = make(map[string]string)
	for _, m := range list {
		k := m.Search(keyName).Data().(string)
		var v string
		if m.Search(valueName) != nil {
			v = m.Search(valueName).Data().(string)
		} else {
			v = m.Search(fallbackValueName).Data().(string)
		}
		result[k] = v
	}

	//	loggit("%s: %v\n", descr, result)
	return
}

func getUsers() (err error) {
	var body []byte

	p := path.Join(inDir, "users.json")
	if body, err = os.ReadFile(p); err != nil {
		log.Printf("ERROR: ReadFile: %s: %v", p, err)
		return
	}

	// we have a user named "wetransfer" (why?) with no "real_name".  Cope.
	users = parseArray(body, "user.list", "id", "real_name", "name")

	//fmt.Printf("users: %v\n", users)
	return
}
func getChannels() (err error) {
	var body []byte

	p := path.Join(inDir, "channels.json")
	if body, err = os.ReadFile(p); err != nil {
		log.Printf("ERROR: ReadFile: %s: %v", p, err)
		return
	}

	channels = parseArray(body, "channel.list", "id", "name", "")

	//fmt.Printf("channels: %v\n", channels)
	return
}

func emitCss(out *os.File) (err error) {
	out.WriteString(`
<style>
body {
	font-family: sans-serif;
}
.msg {
	margin-top: 1em;
}
.msgBody {
	margin-left: 4em;
}
.msgHeader {
    display: flex;
	margin-bottom: 0.5em; 
}
.user {
	font-weight: bold;
}
img {
	max-width: 80%;
}
.ts {
	font-style: italic;
	font-size: smaller;
    margin-left: 1em;
}
</style>
`)
	return
}

// convert slack timestamp ts to a pretty printed string
func tsToPrettyTime(ts string) (result string) {
	v := strings.Split(ts, ".")
	unixTs, err := strconv.ParseInt(v[0], 10, 64)
	if err != nil {
		log.Fatal(err)
	}
	unixTime := time.Unix(unixTs, 0)
	result = unixTime.UTC().Format(time.RFC1123)
	return
}

func formatUsername(userId string) (name string) {
	name = users[userId]
	if name == "" {
		name = userId
	}
	return
}

func processChannelMessage(out *os.File, msg *gabs.Container) (err error) {
	userId := msg.Search("user").Data().(string)
	ts := msg.Search("ts").Data().(string)

	out.WriteString("<div class='msg'>\n")
	out.WriteString("<div class='msgHeader'>")
	out.WriteString("<div class='user'>")
	out.WriteString(formatUsername(userId))
	out.WriteString("</div>\n")
	out.WriteString("<div class='ts'>")
	out.WriteString(tsToPrettyTime(ts))
	out.WriteString("</div>\n")
	out.WriteString("</div>\n")
	out.WriteString("<div class='msgBody'>")

	//         "blocks": [
	//            {                          <--- blockele
	//                "type": "rich_text",
	//                "block_id": "3dbec",
	//                "elements": [          <---- e1
	//                    {                  <---- e1ele
	//                        "type": "rich_text_section",
	//                        "elements": [  <---- e2
	//                            {          <-----ele
	//                                "type": "text",
	//								   "text" "Blah blah"
	//                            }
	//                        ]
	//                    }
	//                ]
	//            }
	//        ]
	blocks := msg.Search("blocks")
	if blocks == nil {
		eleType := msg.Search("type").Data().(string)
		var eleSubtype string
		if msg.Search("subtype") != nil {
			eleSubtype = msg.Search("subtype").Data().(string)
		}
		if eleType == "message" && eleSubtype == "channel_join" {
			out.WriteString("<div class='msgText'>")
			out.WriteString(formatUsername(userId))
			out.WriteString(" has joined the channel")
			out.WriteString("</div>")
		} else if eleType == "message" && eleSubtype == "channel_purpose" {
			out.WriteString("<div class='msgText'>")
			out.WriteString(msg.Search("text").Data().(string))
			out.WriteString("</div>")
		} else {
			//log.Println("DONT KNOW WHAT TO DO: " + eleType + " " + eleSubtype)
			out.WriteString("<div class='msgText'>")
			out.WriteString(msg.Search("text").Data().(string))
			out.WriteString("</div>")
		}
	} else {
		for _, blockele := range blocks.Children() {
			//fmt.Printf("blockele: %s\n", blockele.String())
			e1 := blockele.Search("elements")
			if e1 != nil {
				//fmt.Printf("e1: %s\n", e1.String())
				for _, e1ele := range e1.Children() {
					e2 := e1ele.Search("elements")
					if e2 != nil {
						//fmt.Printf("e2: %s\n", e2.String())

						for _, ele := range e2.Children() {
							//fmt.Printf("ele: %s\n", ele.String())

							eleType := ele.Search("type").Data().(string)
							switch eleType {
							case "channel": // ignore
								out.WriteString("<div class='msgText'>")
								out.WriteString("#")
								channelId := ele.Search("channel_id").Data().(string)
								out.WriteString(channels[channelId])
								out.WriteString("</div>\n")
							case "text":
								out.WriteString("<div class='msgText'>")
								txt := ele.Search("text").Data().(string)
								txt = strings.ReplaceAll(txt, "\n", "<br/>\n")
								out.WriteString(txt)
								out.WriteString("</div>\n")
							case "link":
								out.WriteString("<div class='msgLink'>")
								out.WriteString("<a href='")
								out.WriteString(ele.Search("url").Data().(string))
								out.WriteString("'>")
								out.WriteString(ele.Search("url").Data().(string))
								out.WriteString("</a>")
								out.WriteString("</div>\n")
							}
						}
					}
				}
			}
		}
	}
	attachments := msg.Search("attachments")
	if attachments != nil {
		for _, ele := range attachments.Children() {
			fmt.Printf("attachment: %s\n", ele.String())
			out.WriteString("<a href='")
			out.WriteString(ele.Search("from_url").Data().(string))
			out.WriteString("'><img src='")
			if ele.Search("image_url") != nil {
				out.WriteString(ele.Search("image_url").Data().(string))
			} else if ele.Search("thumb_url") != nil {
				out.WriteString(ele.Search("thumb_url").Data().(string))
			}
			out.WriteString("'></img></a>")
		}
	}
	files := msg.Search("files")
	if files != nil {
		for _, ele := range files.Children() {
			if ele.Search("filetype") != nil {
				filetype := ele.Search("filetype").Data().(string)
				switch filetype {
				case "jpg", "png", "mov", "gif":
					out.WriteString("<img src='")
					out.WriteString(ele.Search("url_private").Data().(string))
					out.WriteString("'></img>")
				default:
					log.Println("DONT KNOW WHAT TO DO files: " + ele.String())
				}
			}
		}
	}
	out.WriteString("</div>\n")
	out.WriteString("</div>\n")

	return
}

func processChannelMessageFile(out *os.File, pathname string) (err error) {
	var body []byte
	if body, err = os.ReadFile(pathname); err != nil {
		return
	}
	jsonParsed, err := gabs.ParseJSON(body)
	if err != nil {
		log.Fatal("FATAL: parse ", "event list", ": ", string(body), ": ", err)
	}

	list := jsonParsed.Children()
	for _, msg := range list {
		if err = processChannelMessage(out, msg); err != nil {
			return
		}
	}
	return
}

func processChannel(name string) (err error) {
	var out *os.File
	if out, err = os.Create(path.Join(outDir, name+".html")); err != nil {
		log.Fatal(err)
	}
	emitHeader(out, name)

	p := path.Join(inDir, name)
	var files []fs.DirEntry
	if files, err = os.ReadDir(p); err != nil {
		log.Printf("ERROR: ReadDir: %s: %v", p, err)
		return
	}
	for _, file := range files {
		fn := path.Join(p, file.Name())
		log.Printf("%s ...\n", fn)
		if err = processChannelMessageFile(out, fn); err != nil {
			return
		}
	}
	emitFooter(out)
	return
}

func processChannels() (err error) {
	// sort by name
	var names []string
	for _, name := range channels {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err = processChannel(name); err != nil {
			return
		}
	}
	return
}

func makeTitle(channelName string) (result string) {
	result = title
	if channelName != "" {
		result = result + " -- #" + channelName
	}
	return
}

func emitHeader(out *os.File, channelName string) (err error) {
	out.WriteString("<html><head>\n")
	if emitCss(out); err != nil {
		return
	}
	out.WriteString("<title>")
	out.WriteString(makeTitle(channelName))
	out.WriteString("</title>")
	out.WriteString("<body>\n")
	out.WriteString("<h1>")
	out.WriteString(makeTitle(channelName))
	out.WriteString("</h1>")
	return
}
func emitFooter(out *os.File) (err error) {
	out.WriteString("</body></html>\n")
	return
}

func processIndex() (err error) {
	var out *os.File
	if out, err = os.Create(path.Join(outDir, "index.html")); err != nil {
		log.Fatal(err)
	}

	if err = emitHeader(out, ""); err != nil {
		return
	}

	// sort by name
	var names []string
	for _, name := range channels {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		out.WriteString("<a href='" + name + ".html'>#" + name + "</a><br>\n")
	}

	if err = emitFooter(out); err != nil {
		return
	}

	return
}
