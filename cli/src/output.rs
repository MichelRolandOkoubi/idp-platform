use clap::ValueEnum;
use comfy_table::{Cell, Color, ContentArrangement, Table};
use serde::Serialize;

#[derive(Debug, Clone, ValueEnum, Default)]
pub enum OutputFormat {
    #[default]
    Text,
    Json,
    Yaml,
    Wide,
}

pub fn print_table(headers: Vec<&str>, rows: Vec<Vec<String>>) {
    let mut table = Table::new();
    table
        .set_content_arrangement(ContentArrangement::Dynamic)
        .set_header(
            headers
                .iter()
                .map(|h| Cell::new(h).fg(Color::Cyan))
                .collect::<Vec<_>>(),
        );
    for row in rows {
        table.add_row(row);
    }
    println!("{table}");
}

pub fn print_json<T: Serialize>(data: &T) {
    println!("{}", serde_json::to_string_pretty(data).unwrap());
}

pub fn print_yaml<T: Serialize>(data: &T) {
    println!("{}", serde_yaml::to_string(data).unwrap());
}

pub fn print_success(msg: &str) {
    use colored::Colorize;
    println!("{} {}", "✓".green().bold(), msg);
}

pub fn print_warning(msg: &str) {
    use colored::Colorize;
    println!("{} {}", "⚠".yellow().bold(), msg);
}

pub fn print_error(msg: &str) {
    use colored::Colorize;
    eprintln!("{} {}", "✗".red().bold(), msg);
}

pub fn print_info(msg: &str) {
    use colored::Colorize;
    println!("{} {}", "→".blue().bold(), msg);
}