#[allow(dead_code)]
mod bowline;

use bowline::{Client, CreateInvoiceInput, Line, ListInvoicesInput, VoidInvoiceInput};
use bowline_client::{Code, Error, Transport};
use std::process::ExitCode;

fn usage() -> ExitCode {
    eprintln!("usage: ledger-rust list | create <description> <quantity> | void <id>");
    ExitCode::from(2)
}

fn client() -> Client {
    let url =
        std::env::var("LEDGER_URL").unwrap_or_else(|_| "http://localhost:8080/api".to_string());
    Client::new(Transport::new(url))
}

fn report(err: Error) -> ExitCode {
    match err {
        Error::Remote(remote) if remote.code == Code::InvalidArgument => {
            eprintln!("rejected: {}", remote.message);
            for issue in remote.issues {
                eprintln!("  {}: {}", issue.path.join("."), issue.message);
            }
        }
        Error::Invalid(issues) => {
            eprintln!("rejected before sending:");
            for issue in issues {
                eprintln!("  {}: {}", issue.path.join("."), issue.message);
            }
        }
        other => eprintln!("error: {other}"),
    }
    ExitCode::FAILURE
}

#[tokio::main]
async fn main() -> ExitCode {
    let args: Vec<String> = std::env::args().skip(1).collect();
    let client = client();
    let result = match args.first().map(String::as_str) {
        Some("list") => list(&client).await,
        Some("create") if args.len() == 3 => match args[2].parse::<i32>() {
            Ok(quantity) => create(&client, &args[1], quantity).await,
            Err(_) => return usage(),
        },
        Some("void") if args.len() == 2 => match args[1].parse::<i64>() {
            Ok(id) => void(&client, id).await,
            Err(_) => return usage(),
        },
        _ => return usage(),
    };
    match result {
        Ok(()) => ExitCode::SUCCESS,
        Err(err) => report(err),
    }
}

async fn list(client: &Client) -> Result<(), Error> {
    let page = client
        .invoices()
        .list(
            &ListInvoicesInput {
                cursor: None,
                limit: 20,
                status: None,
            },
            None,
        )
        .await?;
    for invoice in page.items {
        println!(
            "{}\t{}\t{}\t{}",
            invoice.id,
            invoice.status.as_str(),
            invoice.total,
            invoice.created_at.with_timezone(&chrono::Local)
        );
    }
    Ok(())
}

async fn create(client: &Client, description: &str, quantity: i32) -> Result<(), Error> {
    let invoice = client
        .invoices()
        .create(
            &CreateInvoiceInput {
                customer_id: 1,
                lines: vec![Line {
                    description: description.to_string(),
                    quantity,
                    unit_price: "USD 10.00".to_string(),
                }],
                note: None,
            },
            None,
        )
        .await?;
    println!("created invoice {} for {}", invoice.id, invoice.total);
    Ok(())
}

async fn void(client: &Client, id: i64) -> Result<(), Error> {
    let invoice = client
        .invoices()
        .void(&VoidInvoiceInput { id }, None)
        .await?;
    println!("invoice {} is now {}", invoice.id, invoice.status.as_str());
    Ok(())
}
