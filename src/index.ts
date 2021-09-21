import {
  join,
} from 'path'
import {
  Construct,
  Arn,
  Duration,
} from '@aws-cdk/core'
import {
  Repository,
  RepositoryProps,
} from '@aws-cdk/aws-codecommit'
import {
  GoFunction,
} from '@aws-cdk/aws-lambda-go'
import {
  Grant,
} from '@aws-cdk/aws-iam'
import {
  AfterCreate,
} from 'cdk-triggers'

// ToDo: Use projen (https://www.npmjs.com/package/projen).
// ToDo: Use CDK nag (https://www.npmjs.com/package/cdk-nag).

export interface ForkedRepositoryProps extends RepositoryProps {
  readonly srcRepo: string
}

export class ForkedRepository extends Repository {

  constructor(scope: Construct, id: string, props: ForkedRepositoryProps) {
    super(scope, id, props)
    const entry = join(__dirname, 'fork')
    const timeout = Duration.minutes(5)
    const handler = new GoFunction(this, 'Handler', {
      entry,
      timeout,
    })
    handler.addEnvironment('SRC_REPO', props.srcRepo)
    handler.addEnvironment('DEST_REPO', this.repositoryCloneUrlHttp)
    // ToDo: This needs to be in service patterns.
    const arn = Arn.format({
      service: 'iam',
      resource: 'user',
      region: '',
      resourceName: '*',
    }, this.stack)
    const resourceArns = [
      arn
    ]
    const actions = [
      'iam:CreateUser',
      'iam:CreateServiceSpecificCredential',
      'iam:AttachUserPolicy',
      'iam:DetachUserPolicy',
      'iam:DeleteServiceSpecificCredential',
      'iam:DeleteUser',
    ]
    Grant.addToPrincipal({
      grantee: handler,
      actions,
      resourceArns,
      scope,
    })
    const resources = [
      this,
    ]
    new AfterCreate(this, 'Fork', {
      resources,
      handler,
    })
  }

}
