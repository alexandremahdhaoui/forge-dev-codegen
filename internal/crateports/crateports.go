package crateports

const (
	TicketVerifierPort = "TicketVerifier"
	TokenSourcePort    = "TokenSource"
	SubjectType        = "Subject"

	TicketVerifierModule = "port::ticket_verifier"
	TokenSourceModule    = "port::token_source"
	SubjectModule        = "types::subject"

	TicketVerifierFile = "zz_generated_ticket_verifier.rs"
	TokenSourceFile    = "zz_generated_token_source.rs"
	SubjectFile        = "zz_generated_subject.rs"
)

type Port struct {
	Trait  string
	Module string
	Alias  string
	Layer  string
	File   string
}

func Known() []Port {
	return []Port{
		{Trait: TicketVerifierPort, Module: TicketVerifierModule, Alias: "ticket_verifier", Layer: "port", File: TicketVerifierFile},
		{Trait: TokenSourcePort, Module: TokenSourceModule, Alias: "token_source", Layer: "port", File: TokenSourceFile},
	}
}

func Lookup(trait string) (Port, bool) {
	for _, port := range Known() {
		if port.Trait == trait {
			return port, true
		}
	}

	return Port{}, false
}

func Subject() Port {
	return Port{Trait: SubjectType, Module: SubjectModule, Alias: "subject", Layer: "types", File: SubjectFile}
}

func SubjectSource(header string) string {
	return header + `

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Subject {
    pub id: String,
}
`
}

func TicketVerifierSource(header string) string {
	return header + `

use crate::types::subject::Subject;

#[derive(Debug, thiserror::Error)]
pub enum TicketVerifierError {
    #[error("verifying the ticket: refused: {reason}")]
    Refused { reason: String },
    #[error("verifying the ticket")]
    Verify {
        #[source]
        source: Box<dyn std::error::Error + Send + Sync>,
    },
}

#[cfg_attr(test, mockall::automock)]
pub trait TicketVerifier: Send + Sync {
    fn verify(&self, token: &str) -> Result<Subject, TicketVerifierError>;
}
`
}

func TokenSourceSource(header string) string {
	return header + `

#[derive(Debug, thiserror::Error)]
pub enum TokenSourceError {
    #[error("reading the current token: {reason}")]
    Unavailable { reason: String },
}

#[cfg_attr(test, mockall::automock)]
pub trait TokenSource: Send + Sync {
    fn token(&self) -> Result<String, TokenSourceError>;
}
`
}

func Source(trait, header string) (string, bool) {
	switch trait {
	case TicketVerifierPort:
		return TicketVerifierSource(header), true
	case TokenSourcePort:
		return TokenSourceSource(header), true
	case SubjectType:
		return SubjectSource(header), true
	default:
		return "", false
	}
}
