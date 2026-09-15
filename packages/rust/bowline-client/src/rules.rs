use crate::{Issue, Validate};
use std::collections::BTreeMap;

fn at(path: &[String], field: &str) -> Vec<String> {
    let mut out = path.to_vec();
    out.push(field.to_string());
    out
}

fn push(path: &[String], field: &str, rule: &str, message: String, issues: &mut Vec<Issue>) {
    issues.push(Issue::new(at(path, field), rule, message));
}

pub fn required_str(v: &str, path: &[String], field: &str, issues: &mut Vec<Issue>) {
    if v.is_empty() {
        push(path, field, "required", "is required".into(), issues);
    }
}

pub fn required_int(v: i64, path: &[String], field: &str, issues: &mut Vec<Issue>) {
    if v == 0 {
        push(path, field, "required", "is required".into(), issues);
    }
}

pub fn required_float(v: f64, path: &[String], field: &str, issues: &mut Vec<Issue>) {
    if v == 0.0 {
        push(path, field, "required", "is required".into(), issues);
    }
}

pub fn required_bool(v: bool, path: &[String], field: &str, issues: &mut Vec<Issue>) {
    if !v {
        push(path, field, "required", "is required".into(), issues);
    }
}

pub fn required_len(len: usize, path: &[String], field: &str, issues: &mut Vec<Issue>) {
    if len == 0 {
        push(path, field, "required", "is required".into(), issues);
    }
}

pub fn required_some<T>(v: &Option<T>, path: &[String], field: &str, issues: &mut Vec<Issue>) {
    if v.is_none() {
        push(path, field, "required", "is required".into(), issues);
    }
}

pub fn min_len(
    len: usize,
    min: usize,
    unit: &str,
    path: &[String],
    field: &str,
    issues: &mut Vec<Issue>,
) {
    if len < min {
        push(
            path,
            field,
            "min",
            format!("must be at least {min}{unit}"),
            issues,
        );
    }
}

pub fn max_len(
    len: usize,
    max: usize,
    unit: &str,
    path: &[String],
    field: &str,
    issues: &mut Vec<Issue>,
) {
    if len > max {
        push(
            path,
            field,
            "max",
            format!("must be at most {max}{unit}"),
            issues,
        );
    }
}

pub fn exact_len(
    len: usize,
    want: usize,
    unit: &str,
    path: &[String],
    field: &str,
    issues: &mut Vec<Issue>,
) {
    if len != want {
        push(
            path,
            field,
            "len",
            format!("must be exactly {want}{unit}"),
            issues,
        );
    }
}

pub fn min_num(
    v: f64,
    min: f64,
    param: &str,
    path: &[String],
    field: &str,
    issues: &mut Vec<Issue>,
) {
    if v < min {
        push(
            path,
            field,
            "min",
            format!("must be at least {param}"),
            issues,
        );
    }
}

pub fn max_num(
    v: f64,
    max: f64,
    param: &str,
    path: &[String],
    field: &str,
    issues: &mut Vec<Issue>,
) {
    if v > max {
        push(
            path,
            field,
            "max",
            format!("must be at most {param}"),
            issues,
        );
    }
}

pub fn exact_num(
    v: f64,
    want: f64,
    param: &str,
    path: &[String],
    field: &str,
    issues: &mut Vec<Issue>,
) {
    if v != want {
        push(
            path,
            field,
            "len",
            format!("must be exactly {param}"),
            issues,
        );
    }
}

pub fn one_of(v: &str, options: &[&str], path: &[String], field: &str, issues: &mut Vec<Issue>) {
    if !options.contains(&v) {
        push(
            path,
            field,
            "oneof",
            format!("must be one of {}", options.join(" ")),
            issues,
        );
    }
}

pub fn one_of_int(v: i64, options: &[&str], path: &[String], field: &str, issues: &mut Vec<Issue>) {
    one_of(&v.to_string(), options, path, field, issues);
}

pub fn chars(v: &str) -> usize {
    v.chars().count()
}

pub fn is_email(v: &str) -> bool {
    let Some((local, domain)) = v.split_once('@') else {
        return false;
    };
    if local.is_empty() || domain.is_empty() || domain.contains('@') {
        return false;
    }
    !v.chars()
        .any(|c| c.is_whitespace() || c == '<' || c == '>' || c == '"' || c == ',')
}

pub fn is_url(v: &str) -> bool {
    let Some((scheme, rest)) = v.split_once("://") else {
        return false;
    };
    let scheme_ok = !scheme.is_empty()
        && scheme
            .chars()
            .next()
            .is_some_and(|c| c.is_ascii_alphabetic())
        && scheme
            .chars()
            .all(|c| c.is_ascii_alphanumeric() || c == '+' || c == '-' || c == '.');
    let host = rest.split(['/', '?', '#']).next().unwrap_or("");
    scheme_ok && !host.is_empty()
}

pub fn is_uuid(v: &str) -> bool {
    let groups: Vec<&str> = v.split('-').collect();
    let lengths = [8, 4, 4, 4, 12];
    groups.len() == 5
        && groups
            .iter()
            .zip(lengths.iter())
            .all(|(g, n)| g.len() == *n && g.chars().all(|c| c.is_ascii_hexdigit()))
}

pub fn email(v: &str, path: &[String], field: &str, issues: &mut Vec<Issue>) {
    if !is_email(v) {
        push(
            path,
            field,
            "email",
            "must be a valid email address".into(),
            issues,
        );
    }
}

pub fn url(v: &str, path: &[String], field: &str, issues: &mut Vec<Issue>) {
    if !is_url(v) {
        push(path, field, "url", "must be a valid URL".into(), issues);
    }
}

pub fn uuid(v: &str, path: &[String], field: &str, issues: &mut Vec<Issue>) {
    if !is_uuid(v) {
        push(path, field, "uuid", "must be a valid UUID".into(), issues);
    }
}

pub fn nested<T: Validate + ?Sized>(
    v: &T,
    path: &mut Vec<String>,
    field: &str,
    issues: &mut Vec<Issue>,
) {
    path.push(field.to_string());
    v.validate(path, issues);
    path.pop();
}

pub fn each<T: Validate>(
    items: &[T],
    path: &mut Vec<String>,
    field: &str,
    issues: &mut Vec<Issue>,
) {
    path.push(field.to_string());
    for (i, item) in items.iter().enumerate() {
        path.push(i.to_string());
        item.validate(path, issues);
        path.pop();
    }
    path.pop();
}

pub fn each_map<K: ToString, V: Validate>(
    items: &BTreeMap<K, V>,
    path: &mut Vec<String>,
    field: &str,
    issues: &mut Vec<Issue>,
) {
    path.push(field.to_string());
    for (key, item) in items {
        path.push(key.to_string());
        item.validate(path, issues);
        path.pop();
    }
    path.pop();
}
