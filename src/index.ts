import {
  join,
} from 'path'
import {
  Construct,
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
  CustomUser,
} from '@engr-lynx/cdk-service-patterns'
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
    const user = new CustomUser(this, 'User')
    const entry = join(__dirname, 'fork')
    const timeout = Duration.minutes(5)
    const handler = new GoFunction(this, 'Handler', {
      entry,
      memorySize: 1024,
      timeout,
    })
    handler.addEnvironment('SRC_REPO', props.srcRepo)
    handler.addEnvironment('DEST_REPO', this.repositoryCloneUrlHttp)
    handler.addEnvironment('USER_NAME', user.userName)
    user.grantUpdateCredentials(handler)
    user.grantUpdatePermissions(handler)
    new AfterCreate(this, 'Fork', {
      handler,
    })
  }

}
