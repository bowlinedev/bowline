#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct Event {
    pub name: String,
    pub data: String,
}

#[derive(Debug, Default)]
pub struct Parser {
    buffer: Vec<u8>,
    name: String,
    data: Vec<String>,
}

impl Parser {
    pub fn new() -> Self {
        Parser::default()
    }

    pub fn push(&mut self, chunk: &[u8]) -> Vec<Event> {
        self.buffer.extend_from_slice(chunk);
        let mut events = Vec::new();
        while let Some(end) = self.buffer.iter().position(|b| *b == b'\n') {
            let line: Vec<u8> = self.buffer.drain(..=end).collect();
            let mut text = String::from_utf8_lossy(&line).into_owned();
            if text.ends_with('\n') {
                text.pop();
            }
            if text.ends_with('\r') {
                text.pop();
            }
            if let Some(event) = self.line(&text) {
                events.push(event);
            }
        }
        events
    }

    pub fn finish(&mut self) -> Option<Event> {
        let remaining = std::mem::take(&mut self.buffer);
        if !remaining.is_empty() {
            let text = String::from_utf8_lossy(&remaining).into_owned();
            if let Some(event) = self.line(&text) {
                return Some(event);
            }
        }
        self.flush()
    }

    fn line(&mut self, line: &str) -> Option<Event> {
        if line.is_empty() {
            return self.flush();
        }
        if line.starts_with(':') {
            return None;
        }
        let (field, value) = match line.split_once(':') {
            Some((f, v)) => (f, v.strip_prefix(' ').unwrap_or(v)),
            None => (line, ""),
        };
        match field {
            "event" => self.name = value.to_string(),
            "data" => self.data.push(value.to_string()),
            _ => {}
        }
        None
    }

    fn flush(&mut self) -> Option<Event> {
        if self.name.is_empty() && self.data.is_empty() {
            return None;
        }
        let name = if self.name.is_empty() {
            "message".to_string()
        } else {
            std::mem::take(&mut self.name)
        };
        let data = std::mem::take(&mut self.data).join("\n");
        Some(Event { name, data })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_events_across_chunks_and_skips_comments() {
        let mut parser = Parser::new();
        let first = parser.push(b": open\n\nevent: message\ndata: {\"n\":");
        assert!(first.is_empty());
        let second = parser.push(b"1}\n\n: ping\n\nevent: done\ndata: {}\n\n");
        assert_eq!(
            second,
            vec![
                Event {
                    name: "message".into(),
                    data: "{\"n\":1}".into()
                },
                Event {
                    name: "done".into(),
                    data: "{}".into()
                },
            ]
        );
        assert_eq!(parser.finish(), None);
    }

    #[test]
    fn joins_multiline_data_and_defaults_the_event_name() {
        let mut parser = Parser::new();
        let events = parser.push(b"data: a\r\ndata: b\r\n\r\n");
        assert_eq!(events.len(), 1);
        assert_eq!(events[0].name, "message");
        assert_eq!(events[0].data, "a\nb");
        let tail = parser.push(b"event: error\ndata: {\"error\":{}}");
        assert!(tail.is_empty());
        let last = parser.finish().unwrap();
        assert_eq!(last.name, "error");
    }
}
